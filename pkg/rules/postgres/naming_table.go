package postgres

import (
	"context"
	"fmt"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*NamingTableAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleTableNaming), &NamingTableAdvisor{})
}

// NamingTableAdvisor is the advisor for table naming convention.
type NamingTableAdvisor struct{}

// Check checks the table naming convention.
func (*NamingTableAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	// Get payload
	payload, err := advisor.UnmarshalNamingRulePayloadAsRegexp(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	maxLength := payload.MaxLength
	if maxLength <= 0 {
		maxLength = 64
	}

	checker := &namingTableChecker{
		level:     level,
		title:     string(checkCtx.Rule.Type),
		format:    payload.Format,
		maxLength: maxLength,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type namingTableChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	format     *regexp.Regexp
	maxLength  int
}

// EnterCreatestmt is called when entering a CREATE TABLE statement.
func (c *namingTableChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	allQualifiedNames := ctx.AllQualified_name()
	if len(allQualifiedNames) > 0 {
		tableName := extractTableName(allQualifiedNames[0])
		c.checkTableName(tableName, ctx)
	}
}

// EnterRenamestmt is called when entering an ALTER TABLE RENAME TO statement.
func (c *namingTableChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check for ALTER TABLE ... RENAME TO new_name
	if ctx.TABLE() != nil && ctx.TO() != nil {
		allNames := ctx.AllName()
		if len(allNames) > 0 {
			// The new table name is the last Name() in RENAME TO new_name
			newTableName := pgparser.NormalizePostgreSQLName(allNames[len(allNames)-1])
			c.checkTableName(newTableName, ctx)
		}
	}
}

func (c *namingTableChecker) checkTableName(tableName string, ctx antlr.ParserRuleContext) {
	// Check format
	if !c.format.MatchString(tableName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingTableConvention),
			Title:  c.title,
			Content: fmt.Sprintf(
				`"%s" mismatches table naming convention, naming format should be %q`,
				tableName,
				c.format.String(),
			),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
		return
	}

	// Check length
	if c.maxLength > 0 && len(tableName) > c.maxLength {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingTableConvention),
			Title:  c.title,
			Content: fmt.Sprintf(
				`"%s" mismatches table naming convention, its length should be within %d characters`,
				tableName,
				c.maxLength,
			),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}
