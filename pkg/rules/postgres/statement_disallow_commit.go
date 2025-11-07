package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementDisallowCommitAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementDisallowCommit),
		&StatementDisallowCommitAdvisor{},
	)
}

// StatementDisallowCommitAdvisor is the advisor checking for disallowing COMMIT statements.
type StatementDisallowCommitAdvisor struct{}

// Check checks for disallowing COMMIT statements.
func (*StatementDisallowCommitAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementDisallowCommitChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		statementsText: checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementDisallowCommitChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

// EnterTransactionstmt handles COMMIT statements
func (c *statementDisallowCommitChecker) EnterTransactionstmt(ctx *parser.TransactionstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is a COMMIT statement
	if ctx.COMMIT() == nil {
		return
	}

	stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
	c.adviceList = append(c.adviceList, &types.Advice{
		Status:  c.level,
		Code:    int32(advisor.PostgreSQLDisallowCommit),
		Title:   c.title,
		Content: fmt.Sprintf("Commit is not allowed, related statement: \"%s\"", stmtText),
		StartPosition: &types.Position{
			Line: int32(ctx.GetStart().GetLine()),
		},
	})
}
