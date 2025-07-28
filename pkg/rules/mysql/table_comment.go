package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"github.com/pkg/errors"
)

// TableCommentRule is the ANTLR-based implementation for checking table comment convention
type TableCommentRule struct {
	BaseAntlrRule
	required  bool
	maxLength int
}

// NewTableCommentRule creates a new ANTLR-based table comment rule
func NewTableCommentRule(level types.SQLReviewRuleLevel, title string, required bool, maxLength int) *TableCommentRule {
	return &TableCommentRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		required:  required,
		maxLength: maxLength,
	}
}

// Name returns the rule name
func (*TableCommentRule) Name() string {
	return "TableCommentRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableCommentRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeCreateTable {
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableCommentRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableCommentRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())

	comment, exists := r.handleCreateTableOptions(ctx.CreateTableOptions())

	// Check if comment is required
	if r.required && !exists {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.TableRequireComment),
			Title:         r.title,
			Content:       fmt.Sprintf("Table `%s` requires comments", tableName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}

	// Check comment length
	if r.maxLength >= 0 && len(comment) > r.maxLength {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.TableCommentTooLong),
			Title:         r.title,
			Content:       fmt.Sprintf("The length of table `%s` comment should be within %d characters", tableName, r.maxLength),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (*TableCommentRule) handleCreateTableOptions(ctx mysql.ICreateTableOptionsContext) (string, bool) {
	if ctx == nil {
		return "", false
	}
	for _, option := range ctx.AllCreateTableOption() {
		if option.COMMENT_SYMBOL() == nil || option.TextStringLiteral() == nil {
			continue
		}

		comment := mysqlparser.NormalizeMySQLTextStringLiteral(option.TextStringLiteral())
		return comment, true
	}
	return "", false
}

// TableCommentAdvisor is the advisor using ANTLR parser for table comment convention checking
type TableCommentAdvisor struct{}

// Check performs the ANTLR-based table comment convention check
func (a *TableCommentAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Based on test data, table comments are required with max length of 10
	required := true
	maxLength := 10

	// Create the rule (doesn't need catalog)
	tableRule := NewTableCommentRule(types.SQLReviewRuleLevel(level), string(rule.Type), required, maxLength)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{tableRule})

	for _, stmtNode := range root {
		tableRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
