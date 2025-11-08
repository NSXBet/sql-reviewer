package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementWhereRequireUpdateDeleteAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementRequireWhereForUpdateDelete),
		&StatementWhereRequireUpdateDeleteAdvisor{},
	)
}

// StatementWhereRequireUpdateDeleteAdvisor is the advisor checking for WHERE clause requirement in UPDATE/DELETE.
type StatementWhereRequireUpdateDeleteAdvisor struct{}

// Check checks for WHERE clause requirement in UPDATE/DELETE statements.
func (*StatementWhereRequireUpdateDeleteAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementWhereRequireUpdateDeleteChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		statementsText: checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementWhereRequireUpdateDeleteChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

// EnterUpdatestmt handles UPDATE statements
func (c *statementWhereRequireUpdateDeleteChecker) EnterUpdatestmt(ctx *parser.UpdatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if WHERE clause exists
	if ctx.Where_or_current_clause() == nil || ctx.Where_or_current_clause().WHERE() == nil {
		stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.StatementNoWhere),
			Title:   c.title,
			Content: fmt.Sprintf("\"%s\" requires WHERE clause", stmtText),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

// EnterDeletestmt handles DELETE statements
func (c *statementWhereRequireUpdateDeleteChecker) EnterDeletestmt(ctx *parser.DeletestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if WHERE clause exists
	if ctx.Where_or_current_clause() == nil || ctx.Where_or_current_clause().WHERE() == nil {
		stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.StatementNoWhere),
			Title:   c.title,
			Content: fmt.Sprintf("\"%s\" requires WHERE clause", stmtText),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}
