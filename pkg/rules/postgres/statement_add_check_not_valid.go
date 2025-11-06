package postgres

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementAddCheckNotValidAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleStatementAddCheckNotValid), &StatementAddCheckNotValidAdvisor{})
}

type StatementAddCheckNotValidAdvisor struct{}

func (*StatementAddCheckNotValidAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementAddCheckNotValidChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementAddCheckNotValidChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

func (c *statementAddCheckNotValidChecker) EnterColconstraint(ctx *parser.ColconstraintContext) {
	parent := ctx.GetParent()
	var alterTableCtx *parser.AltertablestmtContext
	for parent != nil {
		if altCtx, ok := parent.(*parser.AltertablestmtContext); ok {
			alterTableCtx = altCtx
			if !isTopLevel(alterTableCtx.GetParent()) {
				return
			}
			break
		}
		parent = parent.GetParent()
	}

	if alterTableCtx == nil {
		return
	}

	if ctx.Colconstraintelem() != nil {
		constraintElem := ctx.Colconstraintelem()
		if constraintElem.CHECK() != nil {
			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(advisor.PostgreSQLAddCheckNotValid),
				Title:   c.title,
				Content: "Adding check constraints with validation will block reads and writes. You can add check constraints not valid and then validate separately",
				StartPosition: &types.Position{
					Line: int32(alterTableCtx.GetStart().GetLine() - 1),
				},
			})
		}
	}
}

func (c *statementAddCheckNotValidChecker) EnterTableconstraint(ctx *parser.TableconstraintContext) {
	parent := ctx.GetParent()
	var alterTableCtx *parser.AltertablestmtContext
	for parent != nil {
		if altCtx, ok := parent.(*parser.AltertablestmtContext); ok {
			alterTableCtx = altCtx
			if !isTopLevel(alterTableCtx.GetParent()) {
				return
			}
			break
		}
		parent = parent.GetParent()
	}

	if alterTableCtx == nil {
		return
	}

	if ctx.Constraintelem() != nil {
		constraintElem := ctx.Constraintelem()
		if constraintElem.CHECK() != nil {
			hasNotValid := false
			if constraintElem.Constraintattributespec() != nil {
				allAttrs := constraintElem.Constraintattributespec().AllConstraintattributeElem()
				for _, attr := range allAttrs {
					if attr.NOT() != nil && attr.VALID() != nil {
						hasNotValid = true
						break
					}
				}
			}

			if !hasNotValid {
				c.adviceList = append(c.adviceList, &types.Advice{
					Status:  c.level,
					Code:    int32(advisor.PostgreSQLAddCheckNotValid),
					Title:   c.title,
					Content: "Adding check constraints with validation will block reads and writes. You can add check constraints not valid and then validate separately",
					StartPosition: &types.Position{
						Line: int32(alterTableCtx.GetStart().GetLine() - 1),
					},
				})
			}
		}
	}
}
