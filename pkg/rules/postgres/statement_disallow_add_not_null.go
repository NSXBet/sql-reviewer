package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementDisallowAddNotNullAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleStatementDisallowAddNotNull), &StatementDisallowAddNotNullAdvisor{})
}

type StatementDisallowAddNotNullAdvisor struct{}

func (*StatementDisallowAddNotNullAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementDisallowAddNotNullChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementDisallowAddNotNullChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

func (c *statementDisallowAddNotNullChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ALTER() != nil && cmd.SET() != nil && cmd.NOT() != nil && cmd.NULL_P() != nil {
				allColIDs := cmd.AllColid()
				if len(allColIDs) > 0 {
					columnName := pgparser.NormalizePostgreSQLColid(allColIDs[0])
					c.adviceList = append(c.adviceList, &types.Advice{
						Status:  c.level,
						Code:    int32(advisor.PostgreSQLDisallowAddNotNull),
						Title:   c.title,
						Content: fmt.Sprintf("Setting NOT NULL will block reads and writes. You can use CHECK (%q IS NOT NULL) instead", columnName),
						StartPosition: &types.Position{
							Line: int32(ctx.GetStart().GetLine() - 1),
						},
					})
				}
			}
		}
	}
}
