package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnDisallowChangeTypeAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleColumnDisallowChangeType), &ColumnDisallowChangeTypeAdvisor{})
}

type ColumnDisallowChangeTypeAdvisor struct{}

func (*ColumnDisallowChangeTypeAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &columnDisallowChangeTypeChecker{
		level:  level,
		title:  string(checkCtx.Rule.Type),
		tokens: tree.Tokens,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type columnDisallowChangeTypeChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	tokens     *antlr.CommonTokenStream
}

func (c *columnDisallowChangeTypeChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Alter_table_cmds() == nil {
		return
	}

	allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
	for _, cmd := range allCmds {
		if cmd.ALTER() != nil && cmd.TYPE_P() != nil {
			text := c.tokens.GetTextFromRuleContext(ctx)

			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(types.ChangeColumnType),
				Title:   c.title,
				Content: fmt.Sprintf("The statement \"%s\" changes column type", text),
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
			break
		}
	}
}
