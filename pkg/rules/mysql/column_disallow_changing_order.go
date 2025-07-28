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

// ColumnDisallowChangingOrderRule is the ANTLR-based implementation for checking disallow changing column order
type ColumnDisallowChangingOrderRule struct {
	BaseAntlrRule
	text string
}

// NewColumnDisallowChangingOrderRule creates a new ANTLR-based column disallow changing order rule
func NewColumnDisallowChangingOrderRule(level types.SQLReviewRuleLevel, title string) *ColumnDisallowChangingOrderRule {
	return &ColumnDisallowChangingOrderRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnDisallowChangingOrderRule) Name() string {
	return "ColumnDisallowChangingOrderRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnDisallowChangingOrderRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
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
func (*ColumnDisallowChangingOrderRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnDisallowChangingOrderRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

		switch {
		// modify column
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil:
			// do nothing, we just check if Place() is specified
		// change column
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil:
			// do nothing, we just check if Place() is specified
		default:
			continue
		}

		// Check if any placement specification is present (FIRST or AFTER)
		if item.Place() != nil {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.ChangeColumnOrder),
				Title:         r.title,
				Content:       fmt.Sprintf("\"%s\" changes column order", r.text),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// ColumnDisallowChangingOrderAdvisor is the advisor using ANTLR parser for column disallow changing order checking
type ColumnDisallowChangingOrderAdvisor struct{}

// Check performs the ANTLR-based column disallow changing order check
func (a *ColumnDisallowChangingOrderAdvisor) Check(
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

	// Create the rule (doesn't need catalog)
	changingOrderRule := NewColumnDisallowChangingOrderRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{changingOrderRule})

	for _, stmtNode := range root {
		changingOrderRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
