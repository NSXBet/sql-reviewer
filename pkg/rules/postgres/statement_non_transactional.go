package postgres

import (
	"context"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementNonTransactionalAdvisor)(nil)

var (
	// DROP DATABASE [ IF EXISTS ] name [ [ WITH ] ( option [, ...] ) ]
	dropDatabaseReg = regexp.MustCompile(`(?i)DROP DATABASE`)
	// CREATE INDEX CONCURRENTLY cannot run inside a transaction block.
	// CREATE [ UNIQUE ] INDEX [ CONCURRENTLY ] [ [ IF NOT EXISTS ] name ] ON [ ONLY ] table_name [ USING method ] ...
	createIndexReg = regexp.MustCompile(`(?i)CREATE(\s+(UNIQUE\s+)?)INDEX(\s+)CONCURRENTLY`)
	// DROP INDEX CONCURRENTLY cannot run inside a transaction block.
	// DROP INDEX [ CONCURRENTLY ] [ IF EXISTS ] name [, ...] [ CASCADE | RESTRICT ].
	dropIndexReg = regexp.MustCompile(`(?i)DROP(\s+)INDEX(\s+)CONCURRENTLY`)
	// VACUUM cannot run inside a transaction block.
	// VACUUM [ ( option [, ...] ) ] [ table_and_columns [, ...] ]
	// VACUUM [ FULL ] [ FREEZE ] [ VERBOSE ] [ ANALYZE ] [ table_and_columns [, ...] ].
	vacuumReg = regexp.MustCompile(`(?i)^\s*VACUUM`)
)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementNonTransactional),
		&StatementNonTransactionalAdvisor{},
	)
}

// StatementNonTransactionalAdvisor is the advisor checking for non-transactional statements.
type StatementNonTransactionalAdvisor struct{}

// Check checks for non-transactional statements.
func (*StatementNonTransactionalAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &nonTransactionalChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		statementsText: checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type nonTransactionalChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

// checkStatement checks if a statement is non-transactional
func (c *nonTransactionalChecker) checkStatement(ctx antlr.ParserRuleContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
	if isNonTransactionStatement(stmtText) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.StatementNonTransactional),
			Title:   c.title,
			Content: "This statement is non-transactional",
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

// EnterDropdbstmt handles DROP DATABASE
func (c *nonTransactionalChecker) EnterDropdbstmt(ctx *parser.DropdbstmtContext) {
	c.checkStatement(ctx)
}

// EnterIndexstmt handles CREATE INDEX
func (c *nonTransactionalChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	c.checkStatement(ctx)
}

// EnterDropstmt handles DROP INDEX (and other DROP statements)
func (c *nonTransactionalChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	c.checkStatement(ctx)
}

// EnterVacuumstmt handles VACUUM
func (c *nonTransactionalChecker) EnterVacuumstmt(ctx *parser.VacuumstmtContext) {
	c.checkStatement(ctx)
}

// isNonTransactionStatement checks if a statement is non-transactional
func isNonTransactionStatement(stmt string) bool {
	if len(dropDatabaseReg.FindString(stmt)) > 0 {
		return true
	}
	if len(createIndexReg.FindString(stmt)) > 0 {
		return true
	}
	if len(dropIndexReg.FindString(stmt)) > 0 {
		return true
	}
	return len(vacuumReg.FindString(stmt)) > 0
}
