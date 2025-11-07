package postgres

import (
	"context"
	"fmt"
	"slices"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*IndexTotalNumberLimitAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleIndexTotalNumberLimit),
		&IndexTotalNumberLimitAdvisor{},
	)
}

type IndexTotalNumberLimitAdvisor struct{}

func (*IndexTotalNumberLimitAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}
	payload, err := advisor.UnmarshalNumberTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &indexTotalNumberLimitChecker{
		level:     level,
		title:     string(checkCtx.Rule.Type),
		max:       payload.Number,
		catalog:   checkCtx.Catalog.GetFinder(),
		tableLine: make(tableLineMap),
	}

	if checker.catalog.Final.Usable() {
		antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)
	}

	return checker.generateAdvice(), nil
}

type tableLine struct {
	schema string
	table  string
	line   int
}

type tableLineMap map[string]tableLine

func (m tableLineMap) set(schema string, table string, line int) {
	if schema == "" {
		schema = "public"
	}
	m[fmt.Sprintf("%q.%q", schema, table)] = tableLine{
		schema: schema,
		table:  table,
		line:   line,
	}
}

type indexTotalNumberLimitChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	max        int
	catalog    *catalog.Finder
	tableLine  tableLineMap
}

func (c *indexTotalNumberLimitChecker) generateAdvice() []*types.Advice {
	var tableList []tableLine
	for _, table := range c.tableLine {
		tableList = append(tableList, table)
	}
	slices.SortFunc(tableList, func(i, j tableLine) int {
		if i.line < j.line {
			return -1
		}
		if i.line > j.line {
			return 1
		}
		return 0
	})

	for _, table := range tableList {
		tableInfo := c.catalog.Final.FindTable(&catalog.TableFind{
			SchemaName: table.schema,
			TableName:  table.table,
		})
		if tableInfo != nil && tableInfo.CountIndex() > c.max {
			c.adviceList = append(c.adviceList, &types.Advice{
				Status: c.level,
				Code:   int32(advisor.PostgreSQLIndexTotalNumberExceedsLimit),
				Title:  c.title,
				Content: fmt.Sprintf(
					"The count of index in table %q.%q should be no more than %d, but found %d",
					table.schema,
					table.table,
					c.max,
					tableInfo.CountIndex(),
				),
				StartPosition: &types.Position{
					Line: int32(table.line),
				},
			})
		}
	}

	return c.adviceList
}

func (c *indexTotalNumberLimitChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
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

	schemaName := extractSchemaName(qualifiedNames[0])

	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.ColumnDef() != nil {
				if hasIndexConstraint(elem.ColumnDef()) {
					c.tableLine.set(schemaName, tableName, ctx.GetStop().GetLine())
					return
				}
			}

			if elem.Tableconstraint() != nil && hasTableIndexConstraint(elem.Tableconstraint()) {
				c.tableLine.set(schemaName, tableName, ctx.GetStop().GetLine())
				return
			}
		}
	}
}

func (c *indexTotalNumberLimitChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
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

	schemaName := extractSchemaName(ctx.Relation_expr().Qualified_name())

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
				if hasIndexConstraint(cmd.ColumnDef()) {
					c.tableLine.set(schemaName, tableName, ctx.GetStop().GetLine())
					return
				}
			}

			if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
				if hasTableIndexConstraint(cmd.Tableconstraint()) {
					c.tableLine.set(schemaName, tableName, ctx.GetStop().GetLine())
					return
				}
			}
		}
	}
}

func (c *indexTotalNumberLimitChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
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

	schemaName := extractSchemaName(ctx.Relation_expr().Qualified_name())
	c.tableLine.set(schemaName, tableName, ctx.GetStop().GetLine())
}

func hasIndexConstraint(colDef parser.IColumnDefContext) bool {
	if colDef.Colquallist() == nil {
		return false
	}

	allConstraints := colDef.Colquallist().AllColconstraint()
	for _, constraint := range allConstraints {
		if constraint.Colconstraintelem() != nil {
			elem := constraint.Colconstraintelem()
			if elem.PRIMARY() != nil && elem.KEY() != nil {
				return true
			}
			if elem.UNIQUE() != nil {
				return true
			}
		}
	}

	return false
}

func hasTableIndexConstraint(constraint parser.ITableconstraintContext) bool {
	if constraint == nil || constraint.Constraintelem() == nil {
		return false
	}

	elem := constraint.Constraintelem()

	if elem.PRIMARY() != nil && elem.KEY() != nil {
		return true
	}

	if elem.UNIQUE() != nil {
		return true
	}

	return false
}
