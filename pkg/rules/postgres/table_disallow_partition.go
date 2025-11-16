package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*TableDisallowPartitionAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleTableDisallowPartition),
		&TableDisallowPartitionAdvisor{},
	)
}

// TableDisallowPartitionAdvisor is the advisor checking for disallow table partition.
type TableDisallowPartitionAdvisor struct{}

// Check checks for disallow table partition.
func (*TableDisallowPartitionAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &tableDisallowPartitionChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		statementsText:               checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type tableDisallowPartitionChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

// EnterCreatestmt handles CREATE TABLE statements
func (c *tableDisallowPartitionChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this CREATE TABLE has a PARTITION BY clause
	if ctx.Optpartitionspec() != nil && ctx.Optpartitionspec().Partitionspec() != nil {
		stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.CreateTablePartition),
			Title:   c.title,
			Content: fmt.Sprintf("Table partition is forbidden, but \"%s\" creates", stmtText),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

// EnterPartition_cmd handles ALTER TABLE ... ATTACH PARTITION
func (c *tableDisallowPartitionChecker) EnterPartition_cmd(ctx *parser.Partition_cmdContext) {
	if !isTopLevel(ctx.GetParent().GetParent().GetParent()) {
		// Partition_cmd is nested: Altertablestmt -> Alter_table_cmds -> Alter_table_cmd -> Partition_cmd
		return
	}

	// Check for ATTACH PARTITION
	if ctx.ATTACH() != nil && ctx.PARTITION() != nil {
		// Navigate up to get the Altertablestmt context for statement text
		parent := ctx.GetParent()
		for parent != nil {
			if alterTableCtx, ok := parent.(*parser.AltertablestmtContext); ok {
				stmtText := extractStatementText(
					c.statementsText,
					alterTableCtx.GetStart().GetLine(),
					alterTableCtx.GetStop().GetLine(),
				)
				c.adviceList = append(c.adviceList, &types.Advice{
					Status:  c.level,
					Code:    int32(types.CreateTablePartition),
					Title:   c.title,
					Content: fmt.Sprintf("Table partition is forbidden, but \"%s\" creates", stmtText),
					StartPosition: &types.Position{
						Line: int32(alterTableCtx.GetStart().GetLine()),
					},
				})
				break
			}
			parent = parent.GetParent()
		}
	}
}
