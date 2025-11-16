package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementAffectedRowLimitAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementAffectedRowLimit),
		&StatementAffectedRowLimitAdvisor{},
	)
}

type StatementAffectedRowLimitAdvisor struct{}

func (*StatementAffectedRowLimitAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &statementAffectedRowLimitChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		maxRow:                       payload.Number,
		ctx:                          ctx,
		driver:                       checkCtx.Driver,
		usePostgresDatabaseOwner:     checkCtx.UsePostgresDatabaseOwner,
		statementsText:               checkCtx.Statements,
	}

	if payload.Number > 0 && checker.driver != nil {
		antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)
	}

	return checker.adviceList, nil
}

type statementAffectedRowLimitChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList               []*types.Advice
	level                    types.Advice_Status
	title                    string
	maxRow                   int
	driver                   *sql.DB
	ctx                      context.Context
	explainCount             int
	setRoles                 []string
	usePostgresDatabaseOwner bool
	statementsText           string
}

func (c *statementAffectedRowLimitChecker) EnterVariablesetstmt(ctx *parser.VariablesetstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.SET() != nil && ctx.Set_rest() != nil && ctx.Set_rest().Set_rest_more() != nil {
		setRestMore := ctx.Set_rest().Set_rest_more()
		if setRestMore.ROLE() != nil {
			stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
			c.setRoles = append(c.setRoles, stmtText)
		}
	}
}

func (c *statementAffectedRowLimitChecker) EnterUpdatestmt(ctx *parser.UpdatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.checkAffectedRows(ctx)
}

func (c *statementAffectedRowLimitChecker) EnterDeletestmt(ctx *parser.DeletestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.checkAffectedRows(ctx)
}

func (c *statementAffectedRowLimitChecker) checkAffectedRows(ctx antlr.ParserRuleContext) {
	if c.explainCount >= 100 {
		return
	}

	c.explainCount++

	stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
	normalizedStmt := advisor.NormalizeStatement(stmtText)

	res, err := advisor.Query(c.ctx, advisor.QueryContext{
		UsePostgresDatabaseOwner: c.usePostgresDatabaseOwner,
		PreExecutions:            c.setRoles,
	}, c.driver, types.Engine_POSTGRES, fmt.Sprintf("EXPLAIN %s", stmtText))
	if err != nil {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.StatementAffectedRowExceedsLimit),
			Title:   c.title,
			Content: fmt.Sprintf("\"%s\" dry runs failed: %s", normalizedStmt, err.Error()),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
		return
	}

	rowCount, err := getAffectedRows(res)
	if err != nil {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.Internal),
			Title:   c.title,
			Content: fmt.Sprintf("failed to get row count for \"%s\": %s", normalizedStmt, err.Error()),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
		return
	}

	if rowCount > int64(c.maxRow) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.StatementAffectedRowExceedsLimit),
			Title:  c.title,
			Content: fmt.Sprintf(
				"The statement \"%s\" affected %d rows (estimated). The count exceeds %d.",
				normalizedStmt,
				rowCount,
				c.maxRow,
			),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}
