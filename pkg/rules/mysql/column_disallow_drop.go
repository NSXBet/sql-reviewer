package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ColumnDisallowDropRule is the ANTLR-based implementation for checking disallow drop column
type ColumnDisallowDropRule struct {
	BaseAntlrRule
}

// NewColumnDisallowDropRule creates a new ANTLR-based column disallow drop rule
func NewColumnDisallowDropRule(level types.SQLReviewRuleLevel, title string) *ColumnDisallowDropRule {
	return &ColumnDisallowDropRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnDisallowDropRule) Name() string {
	return "ColumnDisallowDropRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnDisallowDropRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeAlterTable {
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnDisallowDropRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnDisallowDropRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil || item.DROP_SYMBOL() == nil || item.ColumnInternalRef() == nil {
			continue
		}

		columnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DropColumn),
			Title:         r.title,
			Content:       fmt.Sprintf("drops column \"%s\" of table \"%s\"", columnName, tableName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + item.GetStart().GetLine()),
		})
	}
}

// ColumnDisallowDropAdvisor is the advisor using ANTLR parser for column disallow drop checking
type ColumnDisallowDropAdvisor struct{}

// Check performs the ANTLR-based column disallow drop check
func (a *ColumnDisallowDropAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	columnDisallowDropRule := NewColumnDisallowDropRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{columnDisallowDropRule})

	for _, stmtNode := range root {
		columnDisallowDropRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
