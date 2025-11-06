package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementDisallowMixInDDLAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleStatementDisallowMixInDDL), &StatementDisallowMixInDDLAdvisor{})
}

type StatementDisallowMixInDDLAdvisor struct{}

func (*StatementDisallowMixInDDLAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	switch checkCtx.ChangeType {
	case types.PlanCheckRunConfig_DDL, types.PlanCheckRunConfig_SDL:
	default:
		return nil, nil
	}

	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementDisallowMixInDDLChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		statementsText:               checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementDisallowMixInDDLChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

func (c *statementDisallowMixInDDLChecker) EnterSelectstmt(ctx *parser.SelectstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDMLAdvice(ctx, "SELECT")
}

func (c *statementDisallowMixInDDLChecker) EnterInsertstmt(ctx *parser.InsertstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDMLAdvice(ctx, "INSERT")
}

func (c *statementDisallowMixInDDLChecker) EnterUpdatestmt(ctx *parser.UpdatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDMLAdvice(ctx, "UPDATE")
}

func (c *statementDisallowMixInDDLChecker) EnterDeletestmt(ctx *parser.DeletestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDMLAdvice(ctx, "DELETE")
}

func (c *statementDisallowMixInDDLChecker) addDMLAdvice(ctx antlr.ParserRuleContext, _ string) {
	startPos := ctx.GetStart().GetStart()
	stopPos := ctx.GetStop().GetStop()

	stmtText := ""
	if stopPos+1 < len(c.statementsText) {
		endPos := stopPos + 1
		for endPos < len(c.statementsText) && c.statementsText[endPos] != ';' {
			endPos++
		}
		if endPos < len(c.statementsText) {
			stmtText = c.statementsText[startPos : endPos+1]
		} else {
			stmtText = c.statementsText[startPos:stopPos+1] + ";"
		}
	} else {
		stmtText = c.statementsText[startPos:stopPos+1] + ";"
	}

	c.adviceList = append(c.adviceList, &types.Advice{
		Status:  c.level,
		Code:    int32(advisor.PostgreSQLStatementDisallowMixInDDL),
		Title:   c.title,
		Content: fmt.Sprintf("Alter schema can only run DDL, %q is not DDL", stmtText),
		StartPosition: &types.Position{
			Line: int32(ctx.GetStart().GetLine()),
		},
	})
}
