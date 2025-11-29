package postgres

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementDisallowAddColumnWithDefaultAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementDisallowAddColumnWithDefault),
		&StatementDisallowAddColumnWithDefaultAdvisor{},
	)
}

type StatementDisallowAddColumnWithDefaultAdvisor struct{}

func (*StatementDisallowAddColumnWithDefaultAdvisor) Check(
	ctx context.Context,
	checkCtx advisor.Context,
) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementDisallowAddColumnWithDefaultChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementDisallowAddColumnWithDefaultChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

func (c *statementDisallowAddColumnWithDefaultChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
				columnDef := cmd.ColumnDef()
				if columnDef.Colquallist() != nil {
					allConstraints := columnDef.Colquallist().AllColconstraint()
					for _, constraint := range allConstraints {
						if constraint.Colconstraintelem() != nil {
							constraintElem := constraint.Colconstraintelem()
							if constraintElem.DEFAULT() != nil {
								c.adviceList = append(c.adviceList, &types.Advice{
									Status:  c.level,
									Code:    int32(types.StatementAddColumnWithDefault),
									Title:   c.title,
									Content: "Adding column with DEFAULT will locked the whole table and rewriting each rows",
									StartPosition: &types.Position{
										Line: int32(ctx.GetStart().GetLine()),
									},
								})
								return
							}
						}
					}
				}
			}
		}
	}
}
