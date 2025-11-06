package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*TableRequirePKAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleTableRequirePK), &TableRequirePKAdvisor{})
}

type TableRequirePKAdvisor struct{}

func (*TableRequirePKAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &tableRequirePKChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		statementsText: checkCtx.Statements,
		catalog:        checkCtx.Catalog.GetFinder(),
		tableMentions:  make(map[string]*tableMention),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	checker.validateFinalState()

	return checker.adviceList, nil
}

type tableMention struct {
	startLine int
	endLine   int
}

type tableRequirePKChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
	catalog        *catalog.Finder

	tableMentions map[string]*tableMention
}

func (c *tableRequirePKChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	var tableName, schemaName string
	allQualifiedNames := ctx.AllQualified_name()
	if len(allQualifiedNames) > 0 {
		tableName = extractTableName(allQualifiedNames[0])
		schemaName = extractSchemaName(allQualifiedNames[0])
		if schemaName == "" {
			schemaName = "public"
		}
	}

	key := fmt.Sprintf("%s.%s", schemaName, tableName)
	c.tableMentions[key] = &tableMention{
		startLine: ctx.GetStart().GetLine(),
		endLine:   ctx.GetStop().GetLine(),
	}
}

func (c *tableRequirePKChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	var tableName, schemaName string
	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		tableName = extractTableName(ctx.Relation_expr().Qualified_name())
		schemaName = extractSchemaName(ctx.Relation_expr().Qualified_name())
		if schemaName == "" {
			schemaName = "public"
		}
	}

	key := fmt.Sprintf("%s.%s", schemaName, tableName)
	c.tableMentions[key] = &tableMention{
		startLine: ctx.GetStart().GetLine(),
		endLine:   ctx.GetStop().GetLine(),
	}
}

func (c *tableRequirePKChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Object_type_any_name() == nil || ctx.Object_type_any_name().TABLE() == nil {
		return
	}

	if ctx.Any_name_list() != nil {
		allNames := ctx.Any_name_list().AllAny_name()
		for _, anyName := range allNames {
			if anyName.Colid() != nil {
				name := pgparser.NormalizePostgreSQLColid(anyName.Colid())
				key := fmt.Sprintf("public.%s", name)
				delete(c.tableMentions, key)
			}
		}
	}
}

func (c *tableRequirePKChecker) validateFinalState() {
	for tableKey, mention := range c.tableMentions {
		schemaName, tableName := parseTableKey(tableKey)

		pk := c.catalog.Final.FindPrimaryKey(&catalog.PrimaryKeyFind{
			SchemaName: schemaName,
			TableName:  tableName,
		})

		if pk == nil {
			content := fmt.Sprintf("Table %q.%q requires PRIMARY KEY", schemaName, tableName)

			statement := extractStatementTextForRequirePK(c.statementsText, mention.startLine, mention.endLine)
			if statement != "" {
				content = fmt.Sprintf("%s, related statement: %q", content, statement)
			}

			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(types.TableRequirePK),
				Title:   c.title,
				Content: content,
				StartPosition: &types.Position{
					Line: int32(mention.startLine),
				},
			})
		}
	}
}

func parseTableKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '.' {
			return key[:i], key[i+1:]
		}
	}
	return "public", key
}

func extractStatementTextForRequirePK(statementsText string, startLine, endLine int) string {
	lines := strings.Split(statementsText, "\n")
	if startLine < 1 || startLine > len(lines) {
		return ""
	}

	startIdx := startLine - 1
	endIdx := endLine - 1

	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	var stmtLines []string
	for i := startIdx; i <= endIdx; i++ {
		stmtLines = append(stmtLines, lines[i])
	}

	return strings.TrimSpace(strings.Join(stmtLines, " "))
}
