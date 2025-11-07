package postgres

import (
	"context"
	"fmt"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*NamingColumnAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleColumnNaming), &NamingColumnAdvisor{})
}

type NamingColumnAdvisor struct{}

func (*NamingColumnAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNamingRulePayloadAsRegexp(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &namingColumnChecker{
		level:     level,
		title:     string(checkCtx.Rule.Type),
		format:    payload.Format,
		maxLength: payload.MaxLength,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type namingColumnChecker struct {
	*parser.BasePostgreSQLParserListener
	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	format     *regexp.Regexp
	maxLength  int
}

// EnterCreatestmt is called when entering a CREATE TABLE statement.
func (c *namingColumnChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Opttableelementlist() == nil {
		return
	}

	// Get table name
	var tableName string
	allQualifiedNames := ctx.AllQualified_name()
	if len(allQualifiedNames) > 0 {
		tableName = extractTableName(allQualifiedNames[0])
	}

	// Check column definitions
	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.ColumnDef() != nil && elem.ColumnDef().Colid() != nil {
				columnName := pgparser.NormalizePostgreSQLColid(elem.ColumnDef().Colid())
				c.checkColumnName(tableName, columnName, elem.ColumnDef().GetStart().GetLine())
			}
		}
	}
}

// EnterAltertablestmt is called when entering an ALTER TABLE statement.
func (c *namingColumnChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Get table name
	var tableName string
	if ctx.Relation_expr() != nil {
		if ctx.Relation_expr().Qualified_name() != nil {
			tableName = extractTableName(ctx.Relation_expr().Qualified_name())
		}
	}

	// Check all alter table commands
	if ctx.Alter_table_cmds() == nil {
		return
	}

	allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
	for _, cmd := range allCmds {
		// ADD COLUMN
		if cmd.ColumnDef() != nil {
			colDef := cmd.ColumnDef()
			if colDef.Colid() != nil {
				columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
				c.checkColumnName(tableName, columnName, cmd.GetStart().GetLine())
			}
		}

		// RENAME COLUMN
		if cmd.COLUMN() != nil {
			allColids := cmd.AllColid()
			if len(allColids) >= 2 {
				// RENAME COLUMN old TO new - check the new name
				newColumnName := pgparser.NormalizePostgreSQLColid(allColids[1])
				c.checkColumnName(tableName, newColumnName, cmd.GetStart().GetLine())
			}
		}
	}
}

// EnterRenamestmt is called when entering a RENAME statement.
func (c *namingColumnChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is RENAME COLUMN
	if ctx.Opt_column() == nil || ctx.Opt_column().COLUMN() == nil {
		return
	}

	// Get table name from the relation
	var tableName string
	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		tableName = extractTableName(ctx.Relation_expr().Qualified_name())
	}
	if tableName == "" {
		return
	}

	// Get the new column name (second name in the RENAME COLUMN statement)
	allNames := ctx.AllName()
	if len(allNames) < 2 {
		return
	}

	newColumnName := pgparser.NormalizePostgreSQLName(allNames[1])
	c.checkColumnName(tableName, newColumnName, ctx.GetStart().GetLine())
}

func (c *namingColumnChecker) checkColumnName(tableName, columnName string, line int) {
	// Check format
	if c.format != nil && !c.format.MatchString(columnName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingColumnConvention),
			Title:  c.title,
			Content: fmt.Sprintf(
				"\"%s\".\"%s\" mismatches column naming convention, naming format should be %q",
				tableName,
				columnName,
				c.format.String(),
			),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
	}

	// Check max length
	if c.maxLength > 0 && len(columnName) > c.maxLength {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingColumnConvention),
			Title:  c.title,
			Content: fmt.Sprintf(
				"\"%s\".\"%s\" mismatches column naming convention, its length should be within %d characters",
				tableName,
				columnName,
				c.maxLength,
			),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
	}
}
