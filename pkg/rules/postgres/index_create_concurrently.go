package postgres

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*IndexConcurrentlyAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleCreateIndexConcurrently), &IndexConcurrentlyAdvisor{})
}

type IndexConcurrentlyAdvisor struct{}

func (*IndexConcurrentlyAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &indexCreateConcurrentlyChecker{
		level:              level,
		title:              string(checkCtx.Rule.Type),
		newlyCreatedTables: make(map[string]bool),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type indexCreateConcurrentlyChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList         []*types.Advice
	level              types.Advice_Status
	title              string
	newlyCreatedTables map[string]bool
}

func (c *indexCreateConcurrentlyChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	qualifiedNames := ctx.AllQualified_name()
	if len(qualifiedNames) > 0 {
		tableName := extractTableName(qualifiedNames[0])
		if tableName != "" {
			c.newlyCreatedTables[tableName] = true
		}
	}
}

func (c *indexCreateConcurrentlyChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	hasConcurrently := ctx.Opt_concurrently() != nil && ctx.Opt_concurrently().CONCURRENTLY() != nil

	if !hasConcurrently {
		if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
			tableName := extractTableName(ctx.Relation_expr().Qualified_name())
			if c.newlyCreatedTables[tableName] {
				return
			}
		}

		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.CreateIndexUnconcurrently),
			Title:   c.title,
			Content: "Creating indexes will block writes on the table, unless use CONCURRENTLY",
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

func (c *indexCreateConcurrentlyChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.INDEX() != nil {
		if ctx.CONCURRENTLY() == nil {
			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(types.CreateIndexUnconcurrently),
				Title:   c.title,
				Content: "Droping indexes will block writes on the table, unless use CONCURRENTLY",
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
		}
	}
}
