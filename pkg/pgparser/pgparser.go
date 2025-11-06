// Package pgparser provides PostgreSQL SQL parsing functionality.
//
// This package wraps the Bytebase PostgreSQL parser to provide consistent
// parsing and normalization functions for use in SQL review rules.
package pgparser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// ParseResult contains the parsed SQL statement tree and tokens.
type ParseResult struct {
	Tree   antlr.Tree
	Tokens *antlr.CommonTokenStream
}

// SyntaxError represents a SQL syntax error with position information.
type SyntaxError struct {
	Message  string
	Position *types.Position
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	if e.Position != nil {
		return fmt.Sprintf("syntax error at line %d, column %d: %s",
			e.Position.Line, e.Position.Column, e.Message)
	}
	return fmt.Sprintf("syntax error: %s", e.Message)
}

// syntaxErrorListener collects syntax errors during parsing.
type syntaxErrorListener struct {
	*antlr.DefaultErrorListener
	err *SyntaxError
}

// SyntaxError is called when a syntax error is encountered.
func (l *syntaxErrorListener) SyntaxError(
	_ antlr.Recognizer,
	_ interface{},
	line, column int,
	msg string,
	_ antlr.RecognitionException,
) {
	if l.err == nil {
		l.err = &SyntaxError{
			Message: msg,
			Position: &types.Position{
				Line:   int32(line),
				Column: int32(column),
			},
		}
	}
}

// ParsePostgreSQL parses a PostgreSQL SQL statement and returns the parse tree.
//
// Example:
//
//	result, err := pgparser.ParsePostgreSQL("CREATE TABLE users (id INT);")
//	if err != nil {
//	    // Handle syntax error
//	}
//	// Use result.Tree for rule checking
func ParsePostgreSQL(sql string) (*ParseResult, error) {
	// Create lexer
	inputStream := antlr.NewInputStream(sql)
	lexer := parser.NewPostgreSQLLexer(inputStream)

	// Setup error listener for lexer
	lexerErrorListener := &syntaxErrorListener{}
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(lexerErrorListener)

	// Create token stream
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Create parser
	p := parser.NewPostgreSQLParser(stream)
	p.BuildParseTrees = true

	// Setup error listener for parser
	parserErrorListener := &syntaxErrorListener{}
	p.RemoveErrorListeners()
	p.AddErrorListener(parserErrorListener)

	// Parse the input
	tree := p.Root()

	// Check for lexer errors
	if lexerErrorListener.err != nil {
		return nil, lexerErrorListener.err
	}

	// Check for parser errors
	if parserErrorListener.err != nil {
		return nil, parserErrorListener.err
	}

	// Check if parse tree is nil
	if tree == nil {
		return nil, &SyntaxError{
			Message: "failed to parse SQL statement",
		}
	}

	return &ParseResult{
		Tree:   tree,
		Tokens: stream,
	}, nil
}

// Normalization functions for PostgreSQL identifiers

// NormalizePostgreSQLQualifiedName normalizes a qualified name (schema.table).
// Returns a slice of name parts (e.g., ["schema", "table"]).
func NormalizePostgreSQLQualifiedName(ctx parser.IQualified_nameContext) []string {
	if ctx == nil {
		return []string{}
	}

	res := []string{NormalizePostgreSQLColid(ctx.Colid())}

	if ctx.Indirection() != nil {
		res = append(res, normalizePostgreSQLIndirection(ctx.Indirection())...)
	}
	return res
}

// normalizePostgreSQLIndirection normalizes indirection elements.
func normalizePostgreSQLIndirection(ctx parser.IIndirectionContext) []string {
	if ctx == nil {
		return []string{}
	}

	var res []string
	for _, child := range ctx.AllIndirection_el() {
		res = append(res, normalizePostgreSQLIndirectionEl(child))
	}
	return res
}

// normalizePostgreSQLIndirectionEl normalizes a single indirection element.
func normalizePostgreSQLIndirectionEl(ctx parser.IIndirection_elContext) string {
	if ctx == nil {
		return ""
	}

	if ctx.DOT() != nil {
		if ctx.STAR() != nil {
			return "*"
		}
		return normalizePostgreSQLAttrName(ctx.Attr_name())
	}
	return ctx.GetText()
}

// normalizePostgreSQLAttrName normalizes an attribute name.
func normalizePostgreSQLAttrName(ctx parser.IAttr_nameContext) string {
	return normalizePostgreSQLCollabel(ctx.Collabel())
}

// normalizePostgreSQLCollabel normalizes a column label.
func normalizePostgreSQLCollabel(ctx parser.ICollabelContext) string {
	if ctx == nil {
		return ""
	}
	if ctx.Identifier() != nil {
		return normalizePostgreSQLIdentifier(ctx.Identifier())
	}
	return toLowerCase(ctx.GetText())
}

// NormalizePostgreSQLColid normalizes a column identifier.
func NormalizePostgreSQLColid(ctx parser.IColidContext) string {
	if ctx == nil {
		return ""
	}

	if ctx.Identifier() != nil {
		return normalizePostgreSQLIdentifier(ctx.Identifier())
	}

	// For non-quote identifier, we just return the lower string for PostgreSQL.
	return toLowerCase(ctx.GetText())
}

// normalizePostgreSQLIdentifier normalizes an identifier.
// Handles quoted and unquoted identifiers according to PostgreSQL rules.
func normalizePostgreSQLIdentifier(ctx parser.IIdentifierContext) string {
	if ctx == nil {
		return ""
	}

	// Handle quoted identifiers
	if ctx.QuotedIdentifier() != nil {
		return normalizePostgreSQLQuotedIdentifier(ctx.QuotedIdentifier().GetText())
	}

	if ctx.UnicodeQuotedIdentifier() != nil {
		return normalizePostgreSQLUnicodeQuotedIdentifier(ctx.UnicodeQuotedIdentifier().GetText())
	}

	// Unquoted identifiers are folded to lowercase
	return toLowerCase(ctx.GetText())
}

// normalizePostgreSQLQuotedIdentifier removes quotes and unescapes doubled quotes.
func normalizePostgreSQLQuotedIdentifier(s string) string {
	if len(s) < 2 {
		return s
	}
	// Remove the quote and unescape the quote.
	// In PostgreSQL, quotes inside quoted identifiers are escaped as ""
	quoted := s[1 : len(s)-1]
	return replaceAll(quoted, `""`, `"`)
}

// normalizePostgreSQLUnicodeQuotedIdentifier handles U&"..." identifiers.
func normalizePostgreSQLUnicodeQuotedIdentifier(s string) string {
	// For simplicity, just remove the U& prefix and treat as quoted identifier
	if len(s) > 3 && s[0] == 'U' && s[1] == '&' && s[2] == '"' {
		return normalizePostgreSQLQuotedIdentifier(s[2:])
	}
	return s
}

// NormalizePostgreSQLName normalizes a name context.
func NormalizePostgreSQLName(ctx parser.INameContext) string {
	if ctx == nil {
		return ""
	}

	if ctx.Colid() != nil {
		return NormalizePostgreSQLColid(ctx.Colid())
	}

	return ""
}

// NormalizePostgreSQLAnyName normalizes an any_name context.
// Returns a slice of name parts.
func NormalizePostgreSQLAnyName(ctx parser.IAny_nameContext) []string {
	if ctx == nil {
		return nil
	}

	var result []string
	result = append(result, NormalizePostgreSQLColid(ctx.Colid()))
	if ctx.Attrs() != nil {
		for _, item := range ctx.Attrs().AllAttr_name() {
			result = append(result, normalizePostgreSQLAttrName(item))
		}
	}

	return result
}

// NormalizePostgreSQLFuncName normalizes a function name.
// Returns a slice of name parts.
func NormalizePostgreSQLFuncName(ctx parser.IFunc_nameContext) []string {
	if ctx == nil {
		return []string{}
	}

	var result []string

	// Handle type_function_name (simple identifiers)
	if ctx.Type_function_name() != nil {
		result = append(result, normalizePostgreSQLTypeFunctionName(ctx.Type_function_name()))
	}

	// Handle qualified function name (colid + indirection)
	if ctx.Colid() != nil {
		result = append(result, NormalizePostgreSQLColid(ctx.Colid()))

		// Handle indirection for qualified names
		if ctx.Indirection() != nil {
			parts := normalizePostgreSQLIndirection(ctx.Indirection())
			result = append(result, parts...)
		}
	}

	// Handle builtin function names
	if ctx.Builtin_function_name() != nil {
		result = append(result, ctx.Builtin_function_name().GetText())
	}

	// Handle special keywords LEFT/RIGHT
	if len(result) == 0 && ctx.GetText() != "" {
		// Fallback for special cases like LEFT, RIGHT keywords
		result = append(result, toLowerCase(ctx.GetText()))
	}

	return result
}

// normalizePostgreSQLTypeFunctionName normalizes a type function name.
func normalizePostgreSQLTypeFunctionName(ctx parser.IType_function_nameContext) string {
	if ctx == nil {
		return ""
	}

	// type_function_name can be identifier, unreserved_keyword, etc.
	text := ctx.GetText()

	// Remove quotes if present and convert to lowercase for normalization
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		// Quoted identifier - preserve case but remove quotes
		return text[1 : len(text)-1]
	}

	// Unquoted identifier - convert to lowercase
	return toLowerCase(text)
}

// Helper functions

// toLowerCase converts a string to lowercase.
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// replaceAll replaces all occurrences of old with new in s.
func replaceAll(s, old, new string) string {
	result := ""
	for {
		idx := indexString(s, old)
		if idx == -1 {
			result += s
			break
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
	return result
}

// indexString returns the index of the first instance of substr in s, or -1 if substr is not present in s.
func indexString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// NormalizeSchemaName normalizes a schema name, returning "public" for empty schemas.
func NormalizeSchemaName(schemaName string) string {
	if schemaName == "" {
		return "public"
	}
	return schemaName
}
