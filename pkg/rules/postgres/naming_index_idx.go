package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*NamingIndexIdxAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleIDXNaming), &NamingIndexIdxAdvisor{})
}

// NamingIndexIdxAdvisor is the advisor for regular index naming convention.
type NamingIndexIdxAdvisor struct{}

// Check checks the regular index naming convention.
func (*NamingIndexIdxAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	format, templateList, maxLength, err := advisor.UnmarshalNamingRulePayloadAsTemplate(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	var catalogFinder *catalog.Finder
	if checkCtx.Catalog != nil {
		catalogFinder = checkCtx.Catalog.GetFinder()
	}

	checker := &namingIndexIdxChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		format:                       format,
		maxLength:                    maxLength,
		templateList:                 templateList,
		catalog:                      catalogFinder,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type namingIndexIdxChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList   []*types.Advice
	level        types.Advice_Status
	title        string
	format       string
	maxLength    int
	templateList []string
	catalog      *catalog.Finder
}

// EnterIndexstmt checks CREATE INDEX statements
func (c *namingIndexIdxChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is a UNIQUE index - if so, skip it
	if ctx.Opt_unique() != nil && ctx.Opt_unique().UNIQUE() != nil {
		return
	}

	// Get index name
	indexName := ""
	if ctx.Name() != nil {
		indexName = pgparser.NormalizePostgreSQLName(ctx.Name())
	}
	if indexName == "" {
		return
	}

	// Get table name
	tableName := ""
	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		tableName = extractTableName(ctx.Relation_expr().Qualified_name())
	}
	if tableName == "" {
		return
	}

	// Get column list
	var columnList []string
	if ctx.Index_params() != nil {
		allParams := ctx.Index_params().AllIndex_elem()
		for _, param := range allParams {
			if param.Colid() != nil {
				colName := pgparser.NormalizePostgreSQLColid(param.Colid())
				columnList = append(columnList, colName)
			}
		}
	}

	c.checkIndexName(indexName, tableName, columnList, ctx.GetStart().GetLine())
}

// EnterRenamestmt checks ALTER INDEX ... RENAME TO statements
func (c *namingIndexIdxChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check for ALTER INDEX ... RENAME TO
	if ctx.INDEX() != nil && ctx.TO() != nil {
		allNames := ctx.AllName()
		if len(allNames) < 1 {
			return
		}

		// Get old index name from qualified_name
		var oldIndexName string
		if ctx.Qualified_name() != nil {
			parts := pgparser.NormalizePostgreSQLQualifiedName(ctx.Qualified_name())
			if len(parts) > 0 {
				oldIndexName = parts[len(parts)-1]
			}
		}

		// Get new index name from the name after TO
		newIndexName := pgparser.NormalizePostgreSQLName(allNames[0])

		// Look up the index in catalog to determine if it's a regular index (not unique, not PK)
		if c.catalog != nil && oldIndexName != "" {
			tableName, index := c.findIndex("", "", oldIndexName)
			if index != nil {
				// Only check if it's a regular index (not unique, not primary)
				if !index.Unique() && !index.Primary() {
					c.checkIndexName(newIndexName, tableName, index.ExpressionList(), ctx.GetStart().GetLine())
				}
			}
		}
	}
}

func (c *namingIndexIdxChecker) checkIndexName(indexName, tableName string, columnList []string, line int) {
	metaData := map[string]string{
		advisor.ColumnListTemplateToken: strings.Join(columnList, "_"),
		advisor.TableNameTemplateToken:  tableName,
	}

	regex, err := c.getTemplateRegexp(metaData)
	if err != nil {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(advisor.Internal),
			Title:   "Internal error for index naming convention rule",
			Content: fmt.Sprintf("Failed to compile regex: %v", err),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
		return
	}

	if !regex.MatchString(indexName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingIndexConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				"Index in table %q mismatches the naming convention, expect %q but found %q",
				tableName,
				regex,
				indexName,
			),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
	}

	if c.maxLength > 0 && len(indexName) > c.maxLength {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingIndexConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				"Index %q in table %q mismatches the naming convention, its length should be within %d characters",
				indexName,
				tableName,
				c.maxLength,
			),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
	}
}

func (c *namingIndexIdxChecker) getTemplateRegexp(tokens map[string]string) (*regexp.Regexp, error) {
	return getTemplateRegexp(c.format, c.templateList, tokens)
}

func (c *namingIndexIdxChecker) findIndex(schemaName string, tableName string, indexName string) (string, *catalog.IndexState) {
	if c.catalog == nil {
		return "", nil
	}
	if schemaName == "" {
		schemaName = "public"
	}
	return c.catalog.Origin.FindIndex(&catalog.IndexFind{
		SchemaName: schemaName,
		TableName:  tableName,
		IndexName:  indexName,
	})
}
