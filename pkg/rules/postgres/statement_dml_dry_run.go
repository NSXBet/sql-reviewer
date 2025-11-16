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

var _ advisor.Advisor = (*StatementDMLDryRunAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleStatementDMLDryRun), &StatementDMLDryRunAdvisor{})
}

// StatementDMLDryRunAdvisor is the advisor checking for DML dry run.
// This rule validates DML statements (INSERT, UPDATE, DELETE) by executing EXPLAIN queries
// against a live database connection within a rolled-back transaction.
//
// The rule requires a database connection (checkCtx.Driver) to function.
// If no connection is available, validation is gracefully skipped.
type StatementDMLDryRunAdvisor struct{}

// Check checks for DML dry run.
func (*StatementDMLDryRunAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	// Only run if we have a database connection
	if checkCtx.Driver == nil {
		// Gracefully skip validation when no connection available
		return nil, nil
	}

	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementDMLDryRunChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		ctx:                          ctx,
		driver:                       checkCtx.Driver,
		usePostgresDatabaseOwner:     checkCtx.UsePostgresDatabaseOwner,
		statementsText:               checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementDMLDryRunChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList               []*types.Advice
	level                    types.Advice_Status
	title                    string
	driver                   *sql.DB
	ctx                      context.Context
	explainCount             int
	setRoles                 []string
	usePostgresDatabaseOwner bool
	statementsText           string
}

// EnterVariablesetstmt handles SET ROLE statements
func (c *statementDMLDryRunChecker) EnterVariablesetstmt(ctx *parser.VariablesetstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is SET ROLE
	if ctx.SET() != nil && ctx.Set_rest() != nil && ctx.Set_rest().Set_rest_more() != nil {
		setRestMore := ctx.Set_rest().Set_rest_more()
		if setRestMore.ROLE() != nil {
			// Store the SET ROLE statement text
			stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
			c.setRoles = append(c.setRoles, stmtText)
		}
	}
}

// EnterInsertstmt handles INSERT statements
func (c *statementDMLDryRunChecker) EnterInsertstmt(ctx *parser.InsertstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.checkDMLDryRun(ctx)
}

// EnterUpdatestmt handles UPDATE statements
func (c *statementDMLDryRunChecker) EnterUpdatestmt(ctx *parser.UpdatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.checkDMLDryRun(ctx)
}

// EnterDeletestmt handles DELETE statements
func (c *statementDMLDryRunChecker) EnterDeletestmt(ctx *parser.DeletestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.checkDMLDryRun(ctx)
}

func (c *statementDMLDryRunChecker) checkDMLDryRun(ctx antlr.ParserRuleContext) {
	// Check if we've hit the maximum number of EXPLAIN queries
	if c.explainCount >= advisor.MaximumLintExplainSize {
		return
	}

	c.explainCount++

	// Get the statement text
	stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
	normalizedStmt := advisor.NormalizeStatement(stmtText)

	// Run EXPLAIN to perform dry run
	_, err := advisor.Query(c.ctx, advisor.QueryContext{
		UsePostgresDatabaseOwner: c.usePostgresDatabaseOwner,
		PreExecutions:            c.setRoles,
	}, c.driver, types.Engine_POSTGRES, fmt.Sprintf("EXPLAIN %s", stmtText))
	if err != nil {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.StatementDMLDryRunFailed),
			Title:   c.title,
			Content: fmt.Sprintf("\"%s\" dry runs failed: %s", normalizedStmt, err.Error()),
			StartPosition: &types.Position{
				Line:   int32(ctx.GetStart().GetLine() - 1), // Convert 1-indexed to 0-indexed
				Column: 0,
			},
		})
	}
}
