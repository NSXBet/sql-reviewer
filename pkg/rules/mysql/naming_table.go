package mysql

import (
	"context"
	"fmt"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// NamingTableAdvisor is the ANTLR-based implementation for checking table naming conventions
type NamingTableAdvisor struct {
	BaseAntlrRule
	text      string
	pattern   *regexp.Regexp
	maxLength int
}

// NewNamingTableAdvisor creates a new ANTLR-based table naming rule
func NewNamingTableAdvisor(level types.SQLReviewRuleLevel, title string, pattern string, maxLength int) *NamingTableAdvisor {
	compiledPattern, _ := regexp.Compile(pattern)
	return &NamingTableAdvisor{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		pattern:   compiledPattern,
		maxLength: maxLength,
	}
}

// Name returns the rule name
func (*NamingTableAdvisor) Name() string {
	return "NamingTableAdvisor"
}

// OnEnter is called when entering a parse tree node
func (r *NamingTableAdvisor) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeRenameTableStatement:
		r.checkRenameTableStatement(ctx.(*mysql.RenameTableStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*NamingTableAdvisor) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *NamingTableAdvisor) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	r.handleTableName(tableName, ctx.GetStart().GetLine())
}

func (r *NamingTableAdvisor) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item.RENAME_SYMBOL() == nil {
			continue
		}
		if item.TableName() == nil {
			continue
		}
		_, tableName := mysqlparser.NormalizeMySQLTableName(item.TableName())
		r.handleTableName(tableName, ctx.GetStart().GetLine())
	}
}

func (r *NamingTableAdvisor) checkRenameTableStatement(ctx *mysql.RenameTableStatementContext) {
	for _, pair := range ctx.AllRenamePair() {
		if pair.TableName() == nil {
			continue
		}
		_, tableName := mysqlparser.NormalizeMySQLTableName(pair.TableName())
		r.handleTableName(tableName, ctx.GetStart().GetLine())
	}
}

func (r *NamingTableAdvisor) handleTableName(tableName string, lineNumber int) {
	lineNumber += r.baseLine
	if r.pattern != nil && !r.pattern.MatchString(tableName) {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.NamingTableConvention),
			Title:         r.title,
			Content:       fmt.Sprintf("`%s` mismatches table naming convention, naming format should be %q", tableName, r.pattern.String()),
			StartPosition: ConvertANTLRLineToPosition(lineNumber),
		})
	}
	if r.maxLength > 0 && len(tableName) > r.maxLength {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.NamingTableConvention),
			Title:         r.title,
			Content:       fmt.Sprintf("`%s` mismatches table naming convention, its length should be within %d characters", tableName, r.maxLength),
			StartPosition: ConvertANTLRLineToPosition(lineNumber),
		})
	}
}

// NamingTableConventionAdvisor is the advisor using ANTLR parser for table naming checking
type NamingTableConventionAdvisor struct{}

// Check performs the ANTLR-based table naming check using payload
func (a *NamingTableConventionAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Parse the regex pattern from rule payload
	payload, err := advisor.UnmarshalStringTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	pattern := payload.String

	// Create the rule with default max length of 64 characters (MySQL limit)
	namingRule := NewNamingTableAdvisor(types.SQLReviewRuleLevel(level), string(rule.Type), pattern, 64)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{namingRule})

	for _, stmtNode := range root {
		namingRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
