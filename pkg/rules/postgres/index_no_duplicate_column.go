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

var _ advisor.Advisor = (*IndexNoDuplicateColumnAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleIndexNoDuplicateColumn),
		&IndexNoDuplicateColumnAdvisor{},
	)
}

type IndexNoDuplicateColumnAdvisor struct{}

func (*IndexNoDuplicateColumnAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &indexNoDuplicateColumnChecker{
		level: level,
		title: string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type indexNoDuplicateColumnChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

func (c *indexNoDuplicateColumnChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	indexName := ""
	if ctx.Name() != nil {
		indexName = pgparser.NormalizePostgreSQLName(ctx.Name())
	}

	tableName := ""
	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		tableName = extractTableName(ctx.Relation_expr().Qualified_name())
	}

	if ctx.Index_params() != nil {
		columns := c.extractIndexColumns(ctx.Index_params())
		if dupCol := findDuplicate(columns); dupCol != "" {
			c.addAdvice("INDEX", indexName, tableName, dupCol, ctx.GetStart().GetLine())
		}
	}
}

func (c *indexNoDuplicateColumnChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
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
			if elem.Tableconstraint() != nil {
				c.checkTableConstraint(elem.Tableconstraint(), tableName, elem.GetStart().GetLine())
			}
		}
	}
}

func (c *indexNoDuplicateColumnChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
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

func (c *indexNoDuplicateColumnChecker) checkTableConstraint(
	constraint parser.ITableconstraintContext,
	tableName string,
	line int,
) {
	if constraint == nil {
		return
	}

	constraintName := ""
	if constraint.Name() != nil {
		constraintName = pgparser.NormalizePostgreSQLName(constraint.Name())
	}

	if constraint.Constraintelem() != nil {
		elem := constraint.Constraintelem()

		if elem.PRIMARY() != nil && elem.KEY() != nil {
			if elem.Columnlist() != nil {
				columns := c.extractColumnList(elem.Columnlist())
				if dupCol := findDuplicate(columns); dupCol != "" {
					c.addAdvice("PRIMARY KEY", constraintName, tableName, dupCol, line)
				}
			}
		}

		if elem.UNIQUE() != nil {
			if elem.Columnlist() != nil {
				columns := c.extractColumnList(elem.Columnlist())
				if dupCol := findDuplicate(columns); dupCol != "" {
					c.addAdvice("UNIQUE KEY", constraintName, tableName, dupCol, line)
				}
			}
		}

		if elem.FOREIGN() != nil && elem.KEY() != nil {
			if elem.Columnlist() != nil {
				columns := c.extractColumnList(elem.Columnlist())
				if dupCol := findDuplicate(columns); dupCol != "" {
					c.addAdvice("FOREIGN KEY", constraintName, tableName, dupCol, line)
				}
			}
		}
	}
}

func (*indexNoDuplicateColumnChecker) extractIndexColumns(params parser.IIndex_paramsContext) []string {
	if params == nil {
		return nil
	}

	var columns []string
	allParams := params.AllIndex_elem()
	for _, param := range allParams {
		if param.Colid() != nil {
			colName := pgparser.NormalizePostgreSQLColid(param.Colid())
			columns = append(columns, colName)
		}
	}

	return columns
}

func (*indexNoDuplicateColumnChecker) extractColumnList(columnList parser.IColumnlistContext) []string {
	if columnList == nil {
		return nil
	}

	var columns []string
	allColumns := columnList.AllColumnElem()
	for _, col := range allColumns {
		if col.Colid() != nil {
			colName := pgparser.NormalizePostgreSQLColid(col.Colid())
			columns = append(columns, colName)
		}
	}

	return columns
}

func findDuplicate(columns []string) string {
	seen := make(map[string]bool)
	for _, col := range columns {
		if seen[col] {
			return col
		}
		seen[col] = true
	}
	return ""
}

func (c *indexNoDuplicateColumnChecker) addAdvice(constraintType, constraintName, tableName, duplicateColumn string, line int) {
	c.adviceList = append(c.adviceList, &types.Advice{
		Status:  c.level,
		Code:    int32(types.IndexDuplicateColumn),
		Title:   c.title,
		Content: fmt.Sprintf("%s %q has duplicate column %q.%q", constraintType, constraintName, tableName, duplicateColumn),
		StartPosition: &types.Position{
			Line: int32(line),
		},
	})
}
