package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementInsertRowLimitAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementInsertRowLimit),
		&StatementInsertRowLimitAdvisor{},
	)
}

// StatementInsertRowLimitAdvisor is the advisor checking for to limit INSERT rows.
type StatementInsertRowLimitAdvisor struct{}

// Check checks for the INSERT row limit.
func (*StatementInsertRowLimitAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &statementInsertRowLimitChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		maxRow:         payload.Number,
		statementsText: checkCtx.Statements,
	}

	if payload.Number > 0 {
		antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)
	}

	return checker.adviceList, nil
}

type statementInsertRowLimitChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	maxRow         int
	statementsText string
}

func (c *statementInsertRowLimitChecker) EnterInsertstmt(ctx *parser.InsertstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is INSERT ... VALUES
	if ctx.Insert_rest() != nil && ctx.Insert_rest().Selectstmt() != nil {
		// Count the number of value lists if this is VALUES
		rowCount := countValueLists(ctx.Insert_rest().Selectstmt())
		if rowCount > 0 && rowCount > c.maxRow {
			// This is INSERT ... VALUES
			stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(advisor.PostgreSQLInsertRowLimitExceeds),
				Title:   c.title,
				Content: fmt.Sprintf("The statement \"%s\" inserts %d rows. The count exceeds %d.", stmtText, rowCount, c.maxRow),
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
		}
	}
}

// countValueLists counts the number of value lists in an INSERT ... VALUES statement
// Returns 0 if this is not a VALUES statement (e.g., INSERT ... SELECT)
func countValueLists(selectStmt parser.ISelectstmtContext) int {
	if selectStmt == nil {
		return 0
	}

	// Navigate to the values_clause
	// SELECT can be select_no_parens or select_with_parens
	if selectStmt.Select_no_parens() != nil {
		return countValuesInSelectNoParens(selectStmt.Select_no_parens())
	}

	if selectStmt.Select_with_parens() != nil {
		return countValuesInSelectWithParens(selectStmt.Select_with_parens())
	}

	return 0
}

// countValuesInSelectNoParens counts VALUES rows in a select_no_parens
func countValuesInSelectNoParens(selectCtx parser.ISelect_no_parensContext) int {
	if selectCtx == nil || selectCtx.Select_clause() == nil {
		return 0
	}

	// Check if this is a values_clause
	return countValuesInSelectClause(selectCtx.Select_clause())
}

// countValuesInSelectWithParens counts VALUES rows in a select_with_parens
func countValuesInSelectWithParens(selectCtx parser.ISelect_with_parensContext) int {
	if selectCtx == nil {
		return 0
	}

	if selectCtx.Select_no_parens() != nil {
		return countValuesInSelectNoParens(selectCtx.Select_no_parens())
	}

	if selectCtx.Select_with_parens() != nil {
		return countValuesInSelectWithParens(selectCtx.Select_with_parens())
	}

	return 0
}

// countValuesInSelectClause counts VALUES rows in a select_clause
func countValuesInSelectClause(selectClause parser.ISelect_clauseContext) int {
	if selectClause == nil {
		return 0
	}

	// select_clause has AllSimple_select_intersect
	allIntersect := selectClause.AllSimple_select_intersect()
	if len(allIntersect) == 0 {
		return 0
	}

	// Check the first one for values_clause
	return countValuesInSimpleSelectIntersect(allIntersect[0])
}

// countValuesInSimpleSelectIntersect counts VALUES rows in simple_select_intersect
func countValuesInSimpleSelectIntersect(intersect parser.ISimple_select_intersectContext) int {
	if intersect == nil {
		return 0
	}

	// Get all simple_select_pramary
	allPrimary := intersect.AllSimple_select_pramary()
	if len(allPrimary) == 0 {
		return 0
	}

	// Check the first one for values_clause
	return countValuesInPrimary(allPrimary[0])
}

// countValuesInPrimary counts VALUES rows in simple_select_pramary
func countValuesInPrimary(primary parser.ISimple_select_pramaryContext) int {
	if primary == nil || primary.Values_clause() == nil {
		return 0
	}

	// values_clause: VALUES (expr_list) (, (expr_list))*
	// Count the number of COMMA tokens + 1
	valuesClause := primary.Values_clause()
	commaCount := len(valuesClause.AllCOMMA())
	return commaCount + 1
}
