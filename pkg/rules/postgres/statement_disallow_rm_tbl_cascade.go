package postgres

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementDisallowRemoveTblCascadeAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementDisallowRemoveTblCascade),
		&StatementDisallowRemoveTblCascadeAdvisor{},
	)
}

// StatementDisallowRemoveTblCascadeAdvisor is the advisor checking for disallow CASCADE option when removing tables.
type StatementDisallowRemoveTblCascadeAdvisor struct{}

// Check checks for CASCADE option in DROP TABLE and TRUNCATE TABLE statements.
func (*StatementDisallowRemoveTblCascadeAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementDisallowRemoveTblCascadeChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementDisallowRemoveTblCascadeChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

// EnterDropstmt handles DROP TABLE statements with CASCADE
func (c *statementDisallowRemoveTblCascadeChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is a DROP TABLE statement
	if ctx.Object_type_any_name() == nil || ctx.Object_type_any_name().TABLE() == nil {
		return
	}

	// Check for CASCADE option
	if c.hasCascadeOption(ctx.Opt_drop_behavior()) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.StatementDisallowCascade),
			Title:   c.title,
			Content: "The use of CASCADE is not permitted when removing a table",
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

// EnterTruncatestmt handles TRUNCATE TABLE statements with CASCADE
func (c *statementDisallowRemoveTblCascadeChecker) EnterTruncatestmt(ctx *parser.TruncatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check for CASCADE option
	if c.hasCascadeOption(ctx.Opt_drop_behavior()) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.StatementDisallowCascade),
			Title:   c.title,
			Content: "The use of CASCADE is not permitted when removing a table",
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

// hasCascadeOption checks if the drop behavior is CASCADE
func (*statementDisallowRemoveTblCascadeChecker) hasCascadeOption(ctx parser.IOpt_drop_behaviorContext) bool {
	if ctx == nil {
		return false
	}
	return ctx.CASCADE() != nil
}
