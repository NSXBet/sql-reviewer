package postgres

import (
	"context"
	"fmt"
	"slices"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnDefaultDisallowVolatileAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleColumnDefaultDisallowVolatile),
		&ColumnDefaultDisallowVolatileAdvisor{},
	)
}

type ColumnDefaultDisallowVolatileAdvisor struct{}

func (*ColumnDefaultDisallowVolatileAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &columnDefaultDisallowVolatileChecker{
		level:     level,
		title:     string(checkCtx.Rule.Type),
		columnSet: make(map[string]columnData),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.generateAdvice(), nil
}

type columnData struct {
	schema string
	table  string
	name   string
	line   int
}

type columnDefaultDisallowVolatileChecker struct {
	*parser.BasePostgreSQLParserListener

	level      types.Advice_Status
	title      string
	columnSet  map[string]columnData
	adviceList []*types.Advice
}

func (c *columnDefaultDisallowVolatileChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}

	tableName := ctx.Relation_expr().Qualified_name().GetText()
	if tableName == "" {
		return
	}

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
				colDef := cmd.ColumnDef()
				if colDef.Colid() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())

					if c.hasVolatileDefault(colDef) {
						c.addColumn("public", tableName, columnName, colDef.GetStart().GetLine())
					}
				}
			}
		}
	}
}

func (c *columnDefaultDisallowVolatileChecker) hasVolatileDefault(colDef parser.IColumnDefContext) bool {
	if colDef == nil || colDef.Colquallist() == nil {
		return false
	}

	allConstraints := colDef.Colquallist().AllColconstraint()
	for _, constraint := range allConstraints {
		if constraint.Colconstraintelem() != nil {
			elem := constraint.Colconstraintelem()
			if elem.DEFAULT() != nil && elem.B_expr() != nil {
				if c.containsFunctionCall(elem.B_expr()) {
					return true
				}
			}
		}
	}

	return false
}

func (c *columnDefaultDisallowVolatileChecker) containsFunctionCall(expr antlr.Tree) bool {
	if expr == nil {
		return false
	}

	return c.hasFuncExpr(expr)
}

func (c *columnDefaultDisallowVolatileChecker) hasFuncExpr(node antlr.Tree) bool {
	if node == nil {
		return false
	}

	switch node.(type) {
	case *parser.Func_exprContext,
		*parser.Func_expr_windowlessContext,
		*parser.Func_expr_common_subexprContext:
		return true
	}

	if parserRule, ok := node.(antlr.ParserRuleContext); ok {
		for i := 0; i < parserRule.GetChildCount(); i++ {
			child := parserRule.GetChild(i)
			if c.hasFuncExpr(child) {
				return true
			}
		}
	}

	return false
}

func (c *columnDefaultDisallowVolatileChecker) addColumn(schema string, table string, column string, line int) {
	if schema == "" {
		schema = "public"
	}

	c.columnSet[fmt.Sprintf("%s.%s.%s", schema, table, column)] = columnData{
		schema: schema,
		table:  table,
		name:   column,
		line:   line,
	}
}

func (c *columnDefaultDisallowVolatileChecker) generateAdvice() []*types.Advice {
	var columnList []columnData
	for _, column := range c.columnSet {
		columnList = append(columnList, column)
	}
	slices.SortFunc(columnList, func(i, j columnData) int {
		if i.line < j.line {
			return -1
		}
		if i.line > j.line {
			return 1
		}
		return 0
	})

	for _, column := range columnList {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.ColumnRequireDefault),
			Title:   c.title,
			Content: fmt.Sprintf("Column %q.%q in schema %q has volatile DEFAULT", column.table, column.name, column.schema),
			StartPosition: &types.Position{
				Line: int32(column.line),
			},
		})
	}

	return c.adviceList
}
