package postgres

import (
	"context"
	"fmt"
	"slices"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnNoNullAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleColumnNotNull), &ColumnNoNullAdvisor{})
}

type ColumnNoNullAdvisor struct{}

func (*ColumnNoNullAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	var catalogFinder *catalog.Finder
	if checkCtx.Catalog != nil {
		catalogFinder = checkCtx.Catalog.GetFinder()
	}

	checker := &columnNoNullChecker{
		level:           level,
		title:           string(checkCtx.Rule.Type),
		catalog:         catalogFinder,
		nullableColumns: make(columnMap),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.generateAdviceList(), nil
}

type columnName struct {
	schema string
	table  string
	column string
}

func (c columnName) normalizeTableName() string {
	if c.schema == "" || c.schema == "public" {
		return fmt.Sprintf("%q.%q", "public", c.table)
	}
	return fmt.Sprintf("%q.%q", c.schema, c.table)
}

type columnMap map[columnName]int

type columnNoNullChecker struct {
	*parser.BasePostgreSQLParserListener

	level           types.Advice_Status
	title           string
	catalog         *catalog.Finder
	nullableColumns columnMap
}

func (c *columnNoNullChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	tableName := c.extractTableName(ctx.AllQualified_name())
	if tableName == "" {
		return
	}

	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.ColumnDef() != nil {
				colDef := elem.ColumnDef()
				if colDef.Colid() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					c.addColumn("public", tableName, columnName, colDef.GetStart().GetLine())
					c.removeColumnByColConstraints("public", tableName, colDef)
				}
			}

			if elem.Tableconstraint() != nil {
				c.removeColumnByTableConstraint("public", tableName, elem.Tableconstraint())
			}
		}
	}
}

func (c *columnNoNullChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}

	tableName := ctx.Relation_expr().Qualified_name().GetText()

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
				colDef := cmd.ColumnDef()
				if colDef.Colid() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					c.addColumn("public", tableName, columnName, colDef.GetStart().GetLine())
					c.removeColumnByColConstraints("public", tableName, colDef)
				}
			}

			if cmd.ALTER() != nil && cmd.SET() != nil && cmd.NOT() != nil && cmd.NULL_P() != nil {
				allColids := cmd.AllColid()
				if len(allColids) > 0 {
					columnName := pgparser.NormalizePostgreSQLColid(allColids[0])
					c.removeColumn("public", tableName, columnName)
				}
			}

			if cmd.ALTER() != nil && cmd.DROP() != nil && cmd.NOT() != nil && cmd.NULL_P() != nil {
				allColids := cmd.AllColid()
				if len(allColids) > 0 {
					columnName := pgparser.NormalizePostgreSQLColid(allColids[0])
					c.addColumn("public", tableName, columnName, cmd.GetStart().GetLine())
				}
			}

			if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
				c.removeColumnByTableConstraint("public", tableName, cmd.Tableconstraint())
			}
		}
	}
}

func (*columnNoNullChecker) extractTableName(qualifiedNames []parser.IQualified_nameContext) string {
	if len(qualifiedNames) == 0 {
		return ""
	}
	return extractTableName(qualifiedNames[0])
}

func (c *columnNoNullChecker) addColumn(schema, table, column string, line int) {
	if schema == "" {
		schema = "public"
	}
	c.nullableColumns[columnName{schema: schema, table: table, column: column}] = line
}

func (c *columnNoNullChecker) removeColumn(schema, table, column string) {
	if schema == "" {
		schema = "public"
	}
	delete(c.nullableColumns, columnName{schema: schema, table: table, column: column})
}

func (c *columnNoNullChecker) removeColumnByColConstraints(schema, table string, colDef parser.IColumnDefContext) {
	if colDef.Colquallist() == nil {
		return
	}

	columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
	allConstraints := colDef.Colquallist().AllColconstraint()
	for _, constraint := range allConstraints {
		if constraint.Colconstraintelem() == nil {
			continue
		}

		elem := constraint.Colconstraintelem()

		if elem.NOT() != nil && elem.NULL_P() != nil {
			c.removeColumn(schema, table, columnName)
			return
		}

		if elem.PRIMARY() != nil && elem.KEY() != nil {
			c.removeColumn(schema, table, columnName)
			return
		}
	}
}

func (c *columnNoNullChecker) removeColumnByTableConstraint(schema, table string, constraint parser.ITableconstraintContext) {
	if constraint.Constraintelem() == nil {
		return
	}

	elem := constraint.Constraintelem()

	if elem.PRIMARY() != nil && elem.KEY() != nil && elem.Columnlist() != nil {
		allColumnElems := elem.Columnlist().AllColumnElem()
		for _, columnElem := range allColumnElems {
			if columnElem.Colid() != nil {
				c.removeColumn(schema, table, pgparser.NormalizePostgreSQLColid(columnElem.Colid()))
			}
		}
		return
	}

	if elem.PRIMARY() != nil && elem.KEY() != nil && elem.Existingindex() != nil {
		existingIndex := elem.Existingindex()
		if existingIndex.Name() != nil {
			indexName := pgparser.NormalizePostgreSQLName(existingIndex.Name())
			if c.catalog != nil {
				_, index := c.catalog.Origin.FindIndex(&catalog.IndexFind{
					SchemaName: schema,
					TableName:  table,
					IndexName:  indexName,
				})
				if index != nil {
					for _, expression := range index.ExpressionList() {
						c.removeColumn(schema, table, expression)
					}
				}
			}
		}
	}
}

func (c *columnNoNullChecker) generateAdviceList() []*types.Advice {
	var adviceList []*types.Advice
	var columnList []columnName

	for column := range c.nullableColumns {
		columnList = append(columnList, column)
	}

	if len(columnList) > 0 {
		slices.SortFunc(columnList, func(i, j columnName) int {
			if i.schema != j.schema {
				if i.schema < j.schema {
					return -1
				}
				return 1
			}
			if i.table != j.table {
				if i.table < j.table {
					return -1
				}
				return 1
			}
			if i.column < j.column {
				return -1
			}
			if i.column > j.column {
				return 1
			}
			return 0
		})
	}

	for _, column := range columnList {
		adviceList = append(adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.ColumnCannotNull),
			Title:   c.title,
			Content: fmt.Sprintf("Column %q in %s cannot have NULL value", column.column, column.normalizeTableName()),
			StartPosition: &types.Position{
				Line: int32(c.nullableColumns[column]),
			},
		})
	}

	return adviceList
}
