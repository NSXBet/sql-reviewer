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

var _ advisor.Advisor = (*ColumnCommentAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleColumnCommentConvention), &ColumnCommentAdvisor{})
}

// ColumnCommentAdvisor is the advisor for column comment convention.
type ColumnCommentAdvisor struct{}

// Check checks the column comment convention.
func (*ColumnCommentAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalCommentConventionRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &columnCommentChecker{
		level:   level,
		title:   string(checkCtx.Rule.Type),
		payload: payload,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	// Now validate all collected columns against comments
	return checker.generateAdvice(), nil
}

type columnInfo struct {
	schema string
	table  string
	column string
	line   int
}

type commentInfo struct {
	schema  string
	table   string
	column  string
	comment string
	line    int
}

type columnCommentChecker struct {
	*parser.BasePostgreSQLParserListener

	level   types.Advice_Status
	title   string
	payload *advisor.CommentConventionRulePayload

	columns  []columnInfo
	comments []commentInfo
}

// EnterCreatestmt collects columns from CREATE TABLE
func (c *columnCommentChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	tableName := c.extractTableName(ctx.AllQualified_name())
	if tableName == "" {
		return
	}

	// Extract all columns
	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.ColumnDef() != nil && elem.ColumnDef().Colid() != nil {
				columnName := pgparser.NormalizePostgreSQLColid(elem.ColumnDef().Colid())
				c.columns = append(c.columns, columnInfo{
					schema: "public",
					table:  tableName,
					column: columnName,
					line:   elem.ColumnDef().GetStart().GetLine(),
				})
			}
		}
	}
}

// EnterAltertablestmt collects columns from ALTER TABLE ADD COLUMN
func (c *columnCommentChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
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

	// Check ALTER TABLE ADD COLUMN
	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			// ADD COLUMN
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil && cmd.ColumnDef().Colid() != nil {
				columnName := pgparser.NormalizePostgreSQLColid(cmd.ColumnDef().Colid())
				c.columns = append(c.columns, columnInfo{
					schema: "public",
					table:  tableName,
					column: columnName,
					line:   cmd.ColumnDef().GetStart().GetLine(),
				})
			}
		}
	}
}

// EnterCommentstmt collects COMMENT ON COLUMN
func (c *columnCommentChecker) EnterCommentstmt(ctx *parser.CommentstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is a COMMENT ON COLUMN statement
	if ctx.COLUMN() == nil || ctx.Any_name() == nil {
		return
	}

	// Extract table.column name from any_name
	// any_name is like: table.column or schema.table.column
	anyName := ctx.Any_name()
	parts := pgparser.NormalizePostgreSQLAnyName(anyName)
	if len(parts) < 2 {
		return
	}

	tableName := parts[len(parts)-2]
	columnName := parts[len(parts)-1]

	// Extract comment text
	comment := ""
	if ctx.Comment_text() != nil && ctx.Comment_text().Sconst() != nil {
		comment = extractStringConstant(ctx.Comment_text().Sconst())
	}

	c.comments = append(c.comments, commentInfo{
		schema:  "public",
		table:   tableName,
		column:  columnName,
		comment: comment,
		line:    ctx.GetStart().GetLine(),
	})
}

func (*columnCommentChecker) extractTableName(qualifiedNames []parser.IQualified_nameContext) string {
	if len(qualifiedNames) == 0 {
		return ""
	}

	return extractTableName(qualifiedNames[0])
}

func (c *columnCommentChecker) generateAdvice() []*types.Advice {
	var adviceList []*types.Advice

	// For each column, find its comment and validate
	for _, col := range c.columns {
		// Find the last matching comment for this column
		var matchedComment *commentInfo
		for i := range c.comments {
			comment := &c.comments[i]
			if comment.schema == col.schema && comment.table == col.table && comment.column == col.column {
				matchedComment = comment
				// Continue to find the last one
			}
		}

		if matchedComment == nil || matchedComment.comment == "" {
			if c.payload.Required {
				adviceList = append(adviceList, &types.Advice{
					Status:  c.level,
					Code:    int32(types.ColumnRequireComment),
					Title:   c.title,
					Content: fmt.Sprintf("Comment is required for column `%s.%s`", col.table, col.column),
					StartPosition: &types.Position{
						Line: int32(col.line),
					},
				})
			}
		} else {
			comment := matchedComment.comment

			// Check max length
			if c.payload.MaxLength > 0 && len(comment) > c.payload.MaxLength {
				adviceList = append(adviceList, &types.Advice{
					Status:  c.level,
					Code:    int32(types.ColumnCommentTooLong),
					Title:   c.title,
					Content: fmt.Sprintf("Column `%s.%s` comment is too long. The length of comment should be within %d characters", col.table, col.column, c.payload.MaxLength),
					StartPosition: &types.Position{
						Line: int32(matchedComment.line),
					},
				})
			}
		}
	}

	return adviceList
}
