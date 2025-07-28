package mysqlparser

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/mysql-parser"
	
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ParseResult is the result of parsing a MySQL statement.
type ParseResult struct {
	Tree     antlr.Tree
	Tokens   *antlr.CommonTokenStream
	BaseLine int
}

// ParseMySQL parses the given SQL statement and returns the AST.
func ParseMySQL(statement string) ([]*ParseResult, error) {
	statement, err := DealWithDelimiter(statement)
	if err != nil {
		return nil, err
	}
	list, err := parseInputStream(antlr.NewInputStream(statement), statement)
	// HACK(p0ny): the callee may end up in an infinite loop, we print the statement here to help debug.
	if err != nil && strings.Contains(err.Error(), "split SQL statement timed out") {
		slog.Info("split SQL statement timed out", "statement", statement)
	}
	return list, err
}

// DealWithDelimiter converts the delimiter statement to comment, also converts the following statement's delimiter to semicolon(`;`).
func DealWithDelimiter(statement string) (string, error) {
	has, list, err := hasDelimiter(statement)
	if err != nil {
		return "", err
	}
	if has {
		var result []string
		delimiter := `;`
		for _, sql := range list {
			if IsDelimiter(sql.Text) {
				delimiter, err = ExtractDelimiter(sql.Text)
				if err != nil {
					return "", err
				}
				result = append(result, "-- "+sql.Text)
				continue
			}
			// TODO(rebelice): after deal with delimiter, we may cannot get the right line number, fix it.
			if delimiter != ";" && !sql.Empty {
				result = append(result, fmt.Sprintf("%s;", strings.TrimSuffix(sql.Text, delimiter)))
			} else {
				result = append(result, sql.Text)
			}
		}

		statement = strings.Join(result, "\n")
	}
	return statement, nil
}

func parseSingleStatement(baseLine int, statement string) (antlr.Tree, *antlr.CommonTokenStream, error) {
	input := antlr.NewInputStream(statement)
	lexer := parser.NewMySQLLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	p := parser.NewMySQLParser(stream)

	lexerErrorListener := &ParseErrorListener{
		BaseLine:  baseLine,
		Statement: statement,
	}
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(lexerErrorListener)

	parserErrorListener := &ParseErrorListener{
		BaseLine:  baseLine,
		Statement: statement,
	}
	p.RemoveErrorListeners()
	p.AddErrorListener(parserErrorListener)

	p.BuildParseTrees = true

	tree := p.Script()

	if lexerErrorListener.Err != nil {
		return nil, nil, lexerErrorListener.Err
	}

	if parserErrorListener.Err != nil {
		return nil, nil, parserErrorListener.Err
	}

	return tree, stream, nil
}

func mysqlAddSemicolonIfNeeded(sql string) string {
	lexer := parser.NewMySQLLexer(antlr.NewInputStream(sql))
	lexerErrorListener := &ParseErrorListener{
		Statement: sql,
	}
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(lexerErrorListener)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	stream.Fill()
	if lexerErrorListener.Err != nil {
		// If the lexer fails, we cannot add semicolon.
		return sql
	}
	tokens := stream.GetAllTokens()
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].GetChannel() != antlr.TokenDefaultChannel || tokens[i].GetTokenType() == parser.MySQLParserEOF {
			continue
		}

		// The last default channel token is a semicolon.
		if tokens[i].GetTokenType() == parser.MySQLParserSEMICOLON_SYMBOL {
			return sql
		}

		var result []string
		result = append(result, stream.GetTextFromInterval(antlr.NewInterval(0, tokens[i].GetTokenIndex())))
		result = append(result, ";")
		result = append(result, stream.GetTextFromInterval(antlr.NewInterval(tokens[i].GetTokenIndex()+1, tokens[len(tokens)-1].GetTokenIndex())))
		return strings.Join(result, "")
	}
	return sql
}

func parseInputStream(input *antlr.InputStream, statement string) ([]*ParseResult, error) {
	var result []*ParseResult
	lexer := parser.NewMySQLLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	list, err := splitMySQLStatement(stream, statement)
	if err != nil {
		slog.Info("failed to split MySQL statement, use parser instead", "statement", statement)
		// Use parser to split statement.
		list, err = splitByParser(statement, lexer, stream)
		if err != nil {
			return nil, err
		}
	}

	if len(list) > 0 {
		list[len(list)-1].Text = mysqlAddSemicolonIfNeeded(list[len(list)-1].Text)
	}

	baseLine := 0
	for _, s := range list {
		tree, tokens, err := parseSingleStatement(baseLine, s.Text)
		if err != nil {
			return nil, err
		}

		if isEmptyStatement(tokens) {
			continue
		}

		result = append(result, &ParseResult{
			Tree:     tree,
			Tokens:   tokens,
			BaseLine: s.BaseLine,
		})
		baseLine = int(s.End.Line)
	}

	return result, nil
}

func isEmptyStatement(tokens *antlr.CommonTokenStream) bool {
	for _, token := range tokens.GetAllTokens() {
		if token.GetChannel() == antlr.TokenDefaultChannel && token.GetTokenType() != parser.MySQLParserSEMICOLON_SYMBOL && token.GetTokenType() != parser.MySQLParserEOF {
			return false
		}
	}
	return true
}

// IsDelimiter returns true if the statement is a delimiter statement.
func IsDelimiter(stmt string) bool {
	delimiterRegex := `(?i)^\s*DELIMITER\s+`
	re := regexp.MustCompile(delimiterRegex)
	return re.MatchString(stmt)
}

// ExtractDelimiter extracts the delimiter from the delimiter statement.
func ExtractDelimiter(stmt string) (string, error) {
	delimiterRegex := `(?i)^\s*DELIMITER\s+(?P<DELIMITER>[^\s\\]+)\s*`
	re := regexp.MustCompile(delimiterRegex)
	matchList := re.FindStringSubmatch(stmt)
	index := re.SubexpIndex("DELIMITER")
	if index >= 0 && index < len(matchList) {
		return matchList[index], nil
	}
	return "", errors.Errorf("cannot extract delimiter from %q", stmt)
}

// SingleSQL represents a single SQL statement with metadata
type SingleSQL struct {
	Text     string
	BaseLine int
	Start    *types.Position
	End      *types.Position
	Empty    bool
}

func hasDelimiter(statement string) (bool, []SingleSQL, error) {
	// Simple implementation for SQL splitting
	// This would need proper tokenizer implementation for production use
	lines := strings.Split(statement, "\n")
	var result []SingleSQL
	baseLine := 0
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		
		sql := SingleSQL{
			Text:     line,
			BaseLine: baseLine,
			Start: &types.Position{
				Line:   int32(baseLine),
				Column: 0,
			},
			End: &types.Position{
				Line:   int32(i + 1),
				Column: 0,
			},
			Empty:    trimmed == "",
		}
		result = append(result, sql)
		
		if IsDelimiter(line) {
			return true, result, nil
		}
		baseLine = i + 1
	}
	
	return false, result, nil
}



// IsTopMySQLRule returns true if the given context is a top-level MySQL rule.
func IsTopMySQLRule(ctx *antlr.BaseParserRuleContext) bool {
	if ctx.GetParent() == nil {
		return false
	}
	switch ctx.GetParent().(type) {
	case *parser.SimpleStatementContext:
		if ctx.GetParent().GetParent() == nil {
			return false
		}
		if _, ok := ctx.GetParent().GetParent().(*parser.QueryContext); !ok {
			return false
		}
	case *parser.CreateStatementContext, *parser.DropStatementContext, *parser.TransactionOrLockingStatementContext, *parser.AlterStatementContext:
		if ctx.GetParent().GetParent() == nil {
			return false
		}
		if ctx.GetParent().GetParent().GetParent() == nil {
			return false
		}
		if _, ok := ctx.GetParent().GetParent().GetParent().(*parser.QueryContext); !ok {
			return false
		}
	default:
		return false
	}
	return true
}