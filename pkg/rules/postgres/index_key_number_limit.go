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

var _ advisor.Advisor = (*IndexKeyNumberLimitAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleIndexKeyNumberLimit), &IndexKeyNumberLimitAdvisor{})
}

type IndexKeyNumberLimitAdvisor struct{}

func (*IndexKeyNumberLimitAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &indexKeyNumberLimitChecker{
		level: level,
		title: string(checkCtx.Rule.Type),
		max:   payload.Number,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type indexKeyNumberLimitChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	max        int
}

func (c *indexKeyNumberLimitChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Index_params() != nil {
		keyCount := c.countIndexKeys(ctx.Index_params())
		if c.max > 0 && keyCount > c.max {
			indexName := ""
			if ctx.Name() != nil {
				indexName = pgparser.NormalizePostgreSQLName(ctx.Name())
			}

			tableName := ""
			if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
				tableName = extractTableName(ctx.Relation_expr().Qualified_name())
			}

			c.adviceList = append(c.adviceList, &types.Advice{
				Status: c.level,
				Code:   int32(types.IndexKeyNumberExceedsLimit),
				Title:  c.title,
				Content: fmt.Sprintf(
					"The number of keys of index %q in table %q should be not greater than %d",
					indexName,
					tableName,
					c.max,
				),
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
		}
	}
}

func (c *indexKeyNumberLimitChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	qualifiedNames := ctx.AllQualified_name()
	if len(qualifiedNames) == 0 {
		return
	}

	tableName := extractTableName(qualifiedNames[0])
	if tableName == "" {
		return
	}

	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.Tablelikeclause() != nil {
				continue
			}
			if elem.Tableconstraint() != nil {
				c.checkTableConstraint(elem.Tableconstraint(), tableName, ctx.GetStart().GetLine())
			}
		}
	}
}

func (c *indexKeyNumberLimitChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}

	tableName := extractTableName(ctx.Relation_expr().Qualified_name())
	if tableName == "" {
		return
	}

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
				c.checkTableConstraint(cmd.Tableconstraint(), tableName, ctx.GetStart().GetLine())
			}
		}
	}
}

func (c *indexKeyNumberLimitChecker) checkTableConstraint(constraint parser.ITableconstraintContext, tableName string, line int) {
	if constraint == nil {
		return
	}

	var keyCount int
	var constraintName string

	if constraint.Name() != nil {
		constraintName = pgparser.NormalizePostgreSQLName(constraint.Name())
	}

	if constraint.Constraintelem() != nil {
		elem := constraint.Constraintelem()

		if (elem.PRIMARY() != nil && elem.KEY() != nil) || (elem.UNIQUE() != nil) {
			if elem.Columnlist() != nil {
				keyCount = c.countColumnList(elem.Columnlist())
			}
		}

		if elem.FOREIGN() != nil && elem.KEY() != nil {
			if elem.Columnlist() != nil {
				keyCount = c.countColumnList(elem.Columnlist())
			}
		}
	}

	if c.max > 0 && keyCount > c.max {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.IndexKeyNumberExceedsLimit),
			Title:  c.title,
			Content: fmt.Sprintf(
				"The number of keys of index %q in table %q should be not greater than %d",
				constraintName,
				tableName,
				c.max,
			),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
	}
}

func (*indexKeyNumberLimitChecker) countIndexKeys(params parser.IIndex_paramsContext) int {
	if params == nil {
		return 0
	}

	allParams := params.AllIndex_elem()
	return len(allParams)
}

func (*indexKeyNumberLimitChecker) countColumnList(columnList parser.IColumnlistContext) int {
	if columnList == nil {
		return 0
	}

	allColumns := columnList.AllColumnElem()
	return len(allColumns)
}
