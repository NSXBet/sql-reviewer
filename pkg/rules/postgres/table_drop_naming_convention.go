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

var _ advisor.Advisor = (*TableDropNamingConventionAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleTableDropNamingConvention),
		&TableDropNamingConventionAdvisor{},
	)
}

// TableDropNamingConventionAdvisor is the advisor for table drop with naming convention.
type TableDropNamingConventionAdvisor struct{}

// Check checks for table drop with naming convention.
func (*TableDropNamingConventionAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
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

	checker := &tableDropNamingConventionChecker{
		level:        level,
		title:        string(checkCtx.Rule.Type),
		format:       payload.Format,
		formatString: payload.FormatString,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type tableDropNamingConventionChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList   []*types.Advice
	level        types.Advice_Status
	title        string
	format       *regexp.Regexp
	formatString string
}

// EnterDropstmt handles DROP TABLE statements
func (c *tableDropNamingConventionChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is a DROP TABLE statement
	if ctx.Object_type_any_name() == nil || ctx.Object_type_any_name().TABLE() == nil {
		return
	}

	// Check all tables being dropped
	if ctx.Any_name_list() != nil {
		allNames := ctx.Any_name_list().AllAny_name()
		for _, nameCtx := range allNames {
			tableName := c.extractTableNameFromAnyName(nameCtx)
			if tableName != "" && !c.format.MatchString(tableName) {
				c.adviceList = append(c.adviceList, &types.Advice{
					Status: c.level,
					Code:   int32(types.TableDropNamingConventionMismatch),
					Title:  c.title,
					Content: fmt.Sprintf(
						"`%s` mismatches drop table naming convention, naming format should be %q",
						tableName,
						c.formatString,
					),
					StartPosition: &types.Position{
						Line: int32(ctx.GetStart().GetLine()),
					},
				})
			}
		}
	}
}

// extractTableNameFromAnyName extracts the table name from Any_name context.
// For schema.table, returns "table". For just "table", returns "table".
func (*tableDropNamingConventionChecker) extractTableNameFromAnyName(ctx parser.IAny_nameContext) string {
	parts := pgparser.NormalizePostgreSQLAnyName(ctx)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
