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

// TableRequireCharsetRule is the ANTLR-based implementation for checking table require charset
type TableRequireCharsetRule struct {
	BaseAntlrRule
}

// NewTableRequireCharsetRule creates a new ANTLR-based table require charset rule
func NewTableRequireCharsetRule(level types.SQLReviewRuleLevel, title string) *TableRequireCharsetRule {
	return &TableRequireCharsetRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*TableRequireCharsetRule) Name() string {
	return "TableRequireCharsetRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableRequireCharsetRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeCreateTable {
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableRequireCharsetRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableRequireCharsetRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	if tableName == "" {
		return
	}

	hasCharset := false
	if ctx.CreateTableOptions() != nil {
		for _, tableOption := range ctx.CreateTableOptions().AllCreateTableOption() {
			if tableOption.DefaultCharset() != nil {
				hasCharset = true
				break
			}
		}
	}
	if !hasCharset {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.NoCharset),
			Title:         r.title,
			Content:       fmt.Sprintf("Table %s does not have a character set specified", tableName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// TableRequireCharsetAdvisor is the advisor using ANTLR parser for table require charset checking
type TableRequireCharsetAdvisor struct{}

// Check performs the ANTLR-based table require charset check
func (a *TableRequireCharsetAdvisor) Check(
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

	// Create the rule
	tableRequireCharsetRule := NewTableRequireCharsetRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{tableRequireCharsetRule})

	for _, stmtNode := range root {
		tableRequireCharsetRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
