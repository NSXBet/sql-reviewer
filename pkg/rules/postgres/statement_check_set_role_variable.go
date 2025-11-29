package postgres

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementCheckSetRoleVariableAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementCheckSetRoleVariable),
		&StatementCheckSetRoleVariableAdvisor{},
	)
}

// StatementCheckSetRoleVariableAdvisor checks for SET ROLE variable requirements.
type StatementCheckSetRoleVariableAdvisor struct{}

// Check checks for SET ROLE variable requirements.
func (*StatementCheckSetRoleVariableAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementCheckSetRoleVariableChecker{
		level: level,
		title: string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	if !checker.hasSetRole {
		return []*types.Advice{{
			Status:  level,
			Code:    int32(types.StatementCheckSetRoleVariable),
			Title:   checker.title,
			Content: "No SET ROLE statement found.",
		}}, nil
	}

	return nil, nil
}

type statementCheckSetRoleVariableChecker struct {
	*parser.BasePostgreSQLParserListener

	level           types.Advice_Status
	title           string
	hasSetRole      bool
	foundNonSetStmt bool
}

// EnterVariablesetstmt handles SET statements
func (c *statementCheckSetRoleVariableChecker) EnterVariablesetstmt(ctx *parser.VariablesetstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// If we already found a non-SET statement, skip this
	if c.foundNonSetStmt {
		return
	}

	// Check if this is a SET ROLE statement
	setRest := ctx.Set_rest()
	if setRest != nil {
		setRestMore := setRest.Set_rest_more()
		if setRestMore != nil && setRestMore.ROLE() != nil {
			c.hasSetRole = true
		}
	}
}

// EnterEveryRule is called for every rule entry to detect non-SET statements
func (c *statementCheckSetRoleVariableChecker) EnterEveryRule(ctx antlr.ParserRuleContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// If we already found a non-SET statement, no need to continue checking
	if c.foundNonSetStmt {
		return
	}

	// Check if this is a non-SET statement at the top level
	// We only care about statements that are not VariablesetstmtContext
	if _, isSetStmt := ctx.(*parser.VariablesetstmtContext); !isSetStmt {
		// Check if this is a statement node (not structural nodes like Stmt, Root, etc.)
		switch ctx.(type) {
		case *parser.RootContext, *parser.StmtblockContext, *parser.StmtmultiContext, *parser.StmtContext:
			// These are structural nodes, not actual statements
			return
		default:
			// This is a non-SET statement
			c.foundNonSetStmt = true
		}
	}
}
