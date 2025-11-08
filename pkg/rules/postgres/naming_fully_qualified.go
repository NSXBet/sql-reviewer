package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*NamingFullyQualifiedAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleFullyQualifiedObjectName),
		&NamingFullyQualifiedAdvisor{},
	)
}

// NamingFullyQualifiedAdvisor is the advisor for fully qualified object names.
type NamingFullyQualifiedAdvisor struct{}

// Check checks for fully qualified object names.
func (*NamingFullyQualifiedAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &namingFullyQualifiedChecker{
		level: level,
		title: string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type namingFullyQualifiedChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

// EnterCreatestmt handles CREATE TABLE
func (c *namingFullyQualifiedChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	allQualifiedNames := ctx.AllQualified_name()
	if len(allQualifiedNames) > 0 {
		c.checkQualifiedName(allQualifiedNames[0], ctx.GetStop().GetLine())
	}
}

// EnterCreateseqstmt handles CREATE SEQUENCE
func (c *namingFullyQualifiedChecker) EnterCreateseqstmt(ctx *parser.CreateseqstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Qualified_name() != nil {
		c.checkQualifiedName(ctx.Qualified_name(), ctx.GetStop().GetLine())
	}
}

// EnterCreatetrigstmt handles CREATE TRIGGER
func (c *namingFullyQualifiedChecker) EnterCreatetrigstmt(ctx *parser.CreatetrigstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check the table name in the ON clause
	if ctx.Qualified_name() != nil {
		c.checkQualifiedName(ctx.Qualified_name(), ctx.GetStop().GetLine())
	}
}

// EnterIndexstmt handles CREATE INDEX
func (c *namingFullyQualifiedChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check the table name in the ON clause
	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		c.checkQualifiedName(ctx.Relation_expr().Qualified_name(), ctx.GetStop().GetLine())
	}
}

// EnterDropstmt handles DROP TABLE, DROP SEQUENCE, DROP INDEX
func (c *namingFullyQualifiedChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check all qualified names in the drop statement
	if ctx.Any_name_list() != nil {
		for _, anyName := range ctx.Any_name_list().AllAny_name() {
			c.checkAnyName(anyName, ctx.GetStop().GetLine())
		}
	}
}

// EnterAltertablestmt handles ALTER TABLE
func (c *namingFullyQualifiedChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		c.checkQualifiedName(ctx.Relation_expr().Qualified_name(), ctx.GetStop().GetLine())
	}
}

// EnterAlterseqstmt handles ALTER SEQUENCE
func (c *namingFullyQualifiedChecker) EnterAlterseqstmt(ctx *parser.AlterseqstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Qualified_name() != nil {
		c.checkQualifiedName(ctx.Qualified_name(), ctx.GetStop().GetLine())
	}
}

// EnterRenamestmt handles ALTER TABLE RENAME
func (c *namingFullyQualifiedChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		c.checkQualifiedName(ctx.Relation_expr().Qualified_name(), ctx.GetStop().GetLine())
	}
}

// EnterInsertstmt handles INSERT
func (c *namingFullyQualifiedChecker) EnterInsertstmt(ctx *parser.InsertstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Insert_target() != nil && ctx.Insert_target().Qualified_name() != nil {
		c.checkQualifiedName(ctx.Insert_target().Qualified_name(), ctx.GetStop().GetLine())
	}
}

// EnterUpdatestmt handles UPDATE
func (c *namingFullyQualifiedChecker) EnterUpdatestmt(ctx *parser.UpdatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr_opt_alias() != nil && ctx.Relation_expr_opt_alias().Relation_expr() != nil {
		if ctx.Relation_expr_opt_alias().Relation_expr().Qualified_name() != nil {
			c.checkQualifiedName(ctx.Relation_expr_opt_alias().Relation_expr().Qualified_name(), ctx.GetStop().GetLine())
		}
	}
}

// checkQualifiedName checks if a qualified name is fully qualified
func (c *namingFullyQualifiedChecker) checkQualifiedName(ctx parser.IQualified_nameContext, line int) {
	if ctx == nil {
		return
	}

	parts := pgparser.NormalizePostgreSQLQualifiedName(ctx)
	objName := strings.Join(parts, ".")

	if !c.isFullyQualified(objName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.NamingNotFullyQualifiedName),
			Title:   c.title,
			Content: fmt.Sprintf("unqualified object name: '%s'", objName),
			StartPosition: &types.Position{
				Line: int32(line - 1),
			},
		})
	}
}

// checkAnyName checks if an any_name is fully qualified
func (c *namingFullyQualifiedChecker) checkAnyName(ctx parser.IAny_nameContext, line int) {
	if ctx == nil {
		return
	}

	// Extract parts from any_name (schema.object or object)
	parts := pgparser.NormalizePostgreSQLAnyName(ctx)
	objName := strings.Join(parts, ".")

	if !c.isFullyQualified(objName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.NamingNotFullyQualifiedName),
			Title:   c.title,
			Content: fmt.Sprintf("unqualified object name: '%s'", objName),
			StartPosition: &types.Position{
				Line: int32(line - 1),
			},
		})
	}
}

// isFullyQualified checks if an object name is fully qualified (contains a dot)
func (*namingFullyQualifiedChecker) isFullyQualified(objName string) bool {
	if objName == "" {
		return true
	}
	re := regexp.MustCompile(`.+\..+`)
	return re.MatchString(objName)
}
