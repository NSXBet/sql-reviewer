package postgres

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnRequiredAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleRequiredColumn), &ColumnRequiredAdvisor{})
}

type ColumnRequiredAdvisor struct{}

func (*ColumnRequiredAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	columnList, err := advisor.UnmarshalRequiredColumnList(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	requiredColumnsMap := make(columnSet)
	for _, col := range columnList {
		requiredColumnsMap[col] = true
	}

	checker := &columnRequiredChecker{
		level:              level,
		title:              string(checkCtx.Rule.Type),
		requiredColumnsMap: requiredColumnsMap,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type columnSet map[string]bool

type columnRequiredChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList         []*types.Advice
	level              types.Advice_Status
	title              string
	requiredColumnsMap columnSet
	requiredColumns    columnSet
}

func (c *columnRequiredChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	c.requiredColumns = make(columnSet)
	for column := range c.requiredColumnsMap {
		c.requiredColumns[column] = true
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
			if elem.ColumnDef() != nil && elem.ColumnDef().Colid() != nil {
				columnName := pgparser.NormalizePostgreSQLColid(elem.ColumnDef().Colid())
				delete(c.requiredColumns, columnName)
			}
		}
	}

	if len(c.requiredColumns) > 0 {
		var missingColumns []string
		for column := range c.requiredColumns {
			missingColumns = append(missingColumns, column)
		}
		slices.Sort(missingColumns)

		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.ColumnRequired),
			Title:   c.title,
			Content: fmt.Sprintf("Table %q requires columns: %s", tableName, strings.Join(missingColumns, ", ")),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

func (c *columnRequiredChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
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
			if cmd.DROP() != nil {
				allColids := cmd.AllColid()
				if len(allColids) > 0 {
					columnName := pgparser.NormalizePostgreSQLColid(allColids[0])
					if c.requiredColumnsMap[columnName] {
						c.adviceList = append(c.adviceList, &types.Advice{
							Status:  c.level,
							Code:    int32(types.ColumnRequired),
							Title:   c.title,
							Content: fmt.Sprintf("Table %q requires columns: %s", tableName, columnName),
							StartPosition: &types.Position{
								Line: int32(ctx.GetStart().GetLine()),
							},
						})
					}
				}
			}
		}
	}
}

func (c *columnRequiredChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Opt_column() == nil || ctx.Opt_column().COLUMN() == nil {
		return
	}

	var tableName string
	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		tableName = extractTableName(ctx.Relation_expr().Qualified_name())
	}
	if tableName == "" {
		return
	}

	allNames := ctx.AllName()
	if len(allNames) < 2 {
		return
	}

	oldName := pgparser.NormalizePostgreSQLName(allNames[0])
	newName := pgparser.NormalizePostgreSQLName(allNames[1])

	if c.requiredColumnsMap[oldName] && oldName != newName {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.ColumnRequired),
			Title:   c.title,
			Content: fmt.Sprintf("Table %q requires columns: %s", tableName, oldName),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}
