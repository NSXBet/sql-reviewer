package postgres

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementAddFKNotValidAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementAddFKNotValid),
		&StatementAddFKNotValidAdvisor{},
	)
}

type StatementAddFKNotValidAdvisor struct{}

func (*StatementAddFKNotValidAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementAddFKNotValidChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementAddFKNotValidChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

func (c *statementAddFKNotValidChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Alter_table_cmds() == nil {
		return
	}

	allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
	for _, cmd := range allCmds {
		if cmd.ADD_P() == nil {
			continue
		}

		if cmd.Tableconstraint() == nil {
			continue
		}

		constraint := cmd.Tableconstraint()
		if constraint.Constraintelem() == nil {
			continue
		}

		elem := constraint.Constraintelem()

		if elem.FOREIGN() == nil || elem.KEY() == nil {
			continue
		}

		hasNotValid := false
		if elem.Constraintattributespec() != nil {
			allAttrs := elem.Constraintattributespec().AllConstraintattributeElem()
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
				Code:    int32(types.StatementAddFKWithValidation),
				Title:   c.title,
				Content: "Adding foreign keys with validation will block reads and writes. You can add check foreign keys not valid and then validate separately",
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
		}
	}
}
