package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// ColumnDisallowChangeRule is the ANTLR-based implementation for checking disallow CHANGE COLUMN statement
type ColumnDisallowChangeRule struct {
	BaseAntlrRule
	text string
}

// NewColumnDisallowChangeRule creates a new ANTLR-based column disallow change rule
func NewColumnDisallowChangeRule(level types.SQLReviewRuleLevel, title string) *ColumnDisallowChangeRule {
	return &ColumnDisallowChangeRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnDisallowChangeRule) Name() string {
	return "ColumnDisallowChangeRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnDisallowChangeRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnDisallowChangeRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnDisallowChangeRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
		if item == nil {
			continue
		}

		// change column
		if item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.UseChangeColumnStatement),
				Title:         r.title,
				Content:       fmt.Sprintf("\"%s\" contains CHANGE COLUMN statement", r.text),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// ColumnDisallowChangeAdvisor is the advisor using ANTLR parser for column disallow change checking
type ColumnDisallowChangeAdvisor struct{}

// Check performs the ANTLR-based column disallow change check
func (a *ColumnDisallowChangeAdvisor) Check(
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

	// Create the rule (doesn't need catalog)
	disallowChangeRule := NewColumnDisallowChangeRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{disallowChangeRule})

	for _, stmtNode := range root {
		disallowChangeRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
