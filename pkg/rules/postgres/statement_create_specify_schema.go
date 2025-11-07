package postgres

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementCreateSpecifySchemaAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementCreateSpecifySchema),
		&StatementCreateSpecifySchemaAdvisor{},
	)
}

type StatementCreateSpecifySchemaAdvisor struct{}

func (*StatementCreateSpecifySchemaAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementCreateSpecifySchemaChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementCreateSpecifySchemaChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

func (c *statementCreateSpecifySchemaChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	allQualifiedNames := ctx.AllQualified_name()
	if len(allQualifiedNames) > 0 {
		schemaName := extractSchemaName(allQualifiedNames[0])
		if schemaName == "" {
			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(advisor.PostgreSQLStatementCreateSpecifySchema),
				Title:   c.title,
				Content: "Table schema should be specified.",
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine() - 1),
				},
			})
		}
	}
}
