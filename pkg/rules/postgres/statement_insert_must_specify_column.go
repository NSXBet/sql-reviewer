package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementInsertMustSpecifyColumnAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementInsertMustSpecifyColumn),
		&StatementInsertMustSpecifyColumnAdvisor{},
	)
}

// StatementInsertMustSpecifyColumnAdvisor is the advisor checking for to enforce column specified.
type StatementInsertMustSpecifyColumnAdvisor struct{}

// Check checks for to enforce column specified.
func (*StatementInsertMustSpecifyColumnAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementInsertMustSpecifyColumnChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		statementsText: checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementInsertMustSpecifyColumnChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

func (c *statementInsertMustSpecifyColumnChecker) EnterInsertstmt(ctx *parser.InsertstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if column list is specified
	// In PostgreSQL, INSERT has an optional insert_column_list
	if ctx.Insert_rest() == nil {
		return
	}

	// Check if there's an insert_column_list
	hasColumnList := ctx.Insert_rest().Insert_column_list() != nil

	if !hasColumnList {
		stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())

		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.InsertNotSpecifyColumn),
			Title:   c.title,
			Content: fmt.Sprintf("The INSERT statement must specify columns but \"%s\" does not", stmtText),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}
