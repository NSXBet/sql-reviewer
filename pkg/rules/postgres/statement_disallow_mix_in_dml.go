package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementDisallowMixInDMLAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementDisallowMixInDML),
		&StatementDisallowMixInDMLAdvisor{},
	)
}

type StatementDisallowMixInDMLAdvisor struct{}

func (*StatementDisallowMixInDMLAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	switch checkCtx.ChangeType {
	case types.PlanCheckRunConfig_DML:
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

	checker := &statementDisallowMixInDMLChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		statementsText:               checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementDisallowMixInDMLChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

func (c *statementDisallowMixInDMLChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE TABLE")
}

func (c *statementDisallowMixInDMLChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE INDEX")
}

func (c *statementDisallowMixInDMLChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "ALTER TABLE")
}

func (c *statementDisallowMixInDMLChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "DROP")
}

func (c *statementDisallowMixInDMLChecker) EnterCreateschemastmt(ctx *parser.CreateschemastmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE SCHEMA")
}

func (c *statementDisallowMixInDMLChecker) EnterCreateseqstmt(ctx *parser.CreateseqstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE SEQUENCE")
}

func (c *statementDisallowMixInDMLChecker) EnterAlterseqstmt(ctx *parser.AlterseqstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "ALTER SEQUENCE")
}

func (c *statementDisallowMixInDMLChecker) EnterViewstmt(ctx *parser.ViewstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE VIEW")
}

func (c *statementDisallowMixInDMLChecker) EnterCreatefunctionstmt(ctx *parser.CreatefunctionstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE FUNCTION")
}

func (c *statementDisallowMixInDMLChecker) EnterCreatetrigstmt(ctx *parser.CreatetrigstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE TRIGGER")
}

func (c *statementDisallowMixInDMLChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "RENAME")
}

func (c *statementDisallowMixInDMLChecker) EnterAlterobjectschemastmt(ctx *parser.AlterobjectschemastmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "ALTER SET SCHEMA")
}

func (c *statementDisallowMixInDMLChecker) EnterAlterenumstmt(ctx *parser.AlterenumstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "ALTER TYPE")
}

func (c *statementDisallowMixInDMLChecker) EnterAltercompositetypestmt(ctx *parser.AltercompositetypestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "ALTER TYPE")
}

func (c *statementDisallowMixInDMLChecker) EnterCreateextensionstmt(ctx *parser.CreateextensionstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE EXTENSION")
}

func (c *statementDisallowMixInDMLChecker) EnterCreatedbstmt(ctx *parser.CreatedbstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE DATABASE")
}

func (c *statementDisallowMixInDMLChecker) EnterCreatematviewstmt(ctx *parser.CreatematviewstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.addDDLAdvice(ctx, "CREATE MATERIALIZED VIEW")
}

func (c *statementDisallowMixInDMLChecker) addDDLAdvice(ctx antlr.ParserRuleContext, _ string) {
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
		Code:    int32(types.StatementDisallowMixDDLDML),
		Title:   c.title,
		Content: fmt.Sprintf("Data change can only run DML, %q is not DML", stmtText),
		StartPosition: &types.Position{
			Line: int32(ctx.GetStart().GetLine()),
		},
	})
}
