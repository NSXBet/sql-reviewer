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

var _ advisor.Advisor = (*NamingIndexUKAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleUKNaming), &NamingIndexUKAdvisor{})
}

// NamingIndexUKAdvisor is the advisor for unique key naming convention.
type NamingIndexUKAdvisor struct{}

// Check checks the unique key naming convention.
func (*NamingIndexUKAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
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

	var finder *catalog.Finder
	if checkCtx.Catalog != nil {
		finder = checkCtx.Catalog.GetFinder()
	}

	checker := &namingIndexUKChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		format:                       format,
		maxLength:                    maxLength,
		templateList:                 templateList,
		catalog:                      finder,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type namingIndexUKChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList   []*types.Advice
	level        types.Advice_Status
	title        string
	format       string
	maxLength    int
	templateList []string
	catalog      *catalog.Finder
}

// EnterIndexstmt handles CREATE UNIQUE INDEX statements
func (c *namingIndexUKChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Only check UNIQUE indexes
	if ctx.Opt_unique() == nil || ctx.Opt_unique().UNIQUE() == nil {
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

	// Extract column list
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

	metaData := map[string]string{
		advisor.ColumnListTemplateToken: strings.Join(columnList, "_"),
		advisor.TableNameTemplateToken:  tableName,
	}

	c.checkUniqueKeyName(indexName, tableName, metaData, ctx.GetStart().GetLine())
}

// EnterCreatestmt handles CREATE TABLE with UNIQUE constraints
func (c *namingIndexUKChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
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

	// Check table-level constraints
	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.Tableconstraint() != nil {
				c.checkTableConstraint(elem.Tableconstraint(), tableName, elem.GetStart().GetLine())
			}
			// Check column-level constraints
			if elem.ColumnDef() != nil {
				c.checkColumnDef(elem.ColumnDef(), tableName)
			}
		}
	}
}

// EnterAltertablestmt handles ALTER TABLE statements
func (c *namingIndexUKChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
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
			// ADD CONSTRAINT
			if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
				c.checkTableConstraint(cmd.Tableconstraint(), tableName, ctx.GetStart().GetLine())
			}
			// ADD COLUMN with constraints
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
				c.checkColumnDef(cmd.ColumnDef(), tableName)
			}
		}
	}
}

// EnterRenamestmt handles ALTER INDEX ... RENAME TO and ALTER TABLE ... RENAME CONSTRAINT statements
func (c *namingIndexUKChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
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

		// Look up the index in catalog to determine if it's a unique key
		if c.catalog != nil && oldIndexName != "" {
			tableName, index := c.findIndex("", "", oldIndexName)
			if index != nil && index.Unique() && !index.Primary() {
				c.checkUniqueKeyName(newIndexName, tableName, map[string]string{
					advisor.ColumnListTemplateToken: strings.Join(index.ExpressionList(), "_"),
					advisor.TableNameTemplateToken:  tableName,
				}, ctx.GetStart().GetLine())
			}
		}
	}

	// Check for ALTER TABLE ... RENAME CONSTRAINT
	if ctx.CONSTRAINT() != nil && ctx.TO() != nil && c.catalog != nil {
		allNames := ctx.AllName()
		if len(allNames) >= 2 {
			oldConstraintName := pgparser.NormalizePostgreSQLName(allNames[0])
			newConstraintName := pgparser.NormalizePostgreSQLName(allNames[1])

			// Get table name from the statement
			var tableName, schemaName string
			if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
				tableName = extractTableName(ctx.Relation_expr().Qualified_name())
				schemaName = extractSchemaName(ctx.Relation_expr().Qualified_name())
			}

			// Check if this is a unique key constraint in catalog
			foundTableName, index := c.findIndex(schemaName, tableName, oldConstraintName)
			if index != nil && index.Unique() && !index.Primary() {
				metaData := map[string]string{
					advisor.ColumnListTemplateToken: strings.Join(index.ExpressionList(), "_"),
					advisor.TableNameTemplateToken:  foundTableName,
				}
				c.checkUniqueKeyName(newConstraintName, foundTableName, metaData, ctx.GetStart().GetLine())
			}
		}
	}
}

func (c *namingIndexUKChecker) checkTableConstraint(constraint parser.ITableconstraintContext, tableName string, line int) {
	if constraint == nil {
		return
	}

	constraintName := ""
	if constraint.Name() != nil {
		constraintName = pgparser.NormalizePostgreSQLName(constraint.Name())
	}

	if constraint.Constraintelem() != nil {
		elem := constraint.Constraintelem()

		// UNIQUE constraint
		if elem.UNIQUE() != nil {
			var columnList []string
			if elem.Columnlist() != nil {
				allColumns := elem.Columnlist().AllColumnElem()
				for _, col := range allColumns {
					if col.Colid() != nil {
						colName := pgparser.NormalizePostgreSQLColid(col.Colid())
						columnList = append(columnList, colName)
					}
				}
			} else if elem.Existingindex() != nil && elem.Existingindex().Name() != nil {
				// Handle UNIQUE USING INDEX - the column list is in the existing index
				indexName := pgparser.NormalizePostgreSQLName(elem.Existingindex().Name())
				foundTableName, index := c.findIndex("", tableName, indexName)
				if index != nil {
					columnList = index.ExpressionList()
					tableName = foundTableName
				}
			}

			// Only check if we have a constraint name (unnamed constraints are auto-generated)
			if constraintName != "" {
				metaData := map[string]string{
					advisor.ColumnListTemplateToken: strings.Join(columnList, "_"),
					advisor.TableNameTemplateToken:  tableName,
				}
				c.checkUniqueKeyName(constraintName, tableName, metaData, line)
			}
		}
	}
}

func (c *namingIndexUKChecker) checkColumnDef(columndef parser.IColumnDefContext, tableName string) {
	if columndef == nil {
		return
	}

	colName := ""
	if columndef.Colid() != nil {
		colName = pgparser.NormalizePostgreSQLColid(columndef.Colid())
	}

	// Check column-level constraints
	if columndef.Colquallist() != nil {
		allQuals := columndef.Colquallist().AllColconstraint()
		for _, qual := range allQuals {
			if qual.Colconstraintelem() != nil {
				elem := qual.Colconstraintelem()
				// Check for UNIQUE constraint
				if elem.UNIQUE() != nil {
					// Column-level unique constraints with names
					constraintName := ""
					if qual.Name() != nil {
						constraintName = pgparser.NormalizePostgreSQLName(qual.Name())
					}

					// Only check if we have a constraint name
					if constraintName != "" {
						metaData := map[string]string{
							advisor.ColumnListTemplateToken: colName,
							advisor.TableNameTemplateToken:  tableName,
						}
						c.checkUniqueKeyName(constraintName, tableName, metaData, qual.GetStart().GetLine())
					}
				}
			}
		}
	}
}

func (c *namingIndexUKChecker) checkUniqueKeyName(indexName, tableName string, metaData map[string]string, line int) {
	regex, err := c.getTemplateRegexp(metaData)
	if err != nil {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.Internal),
			Title:   "Internal error for unique key naming convention rule",
			Content: fmt.Sprintf("Failed to compile regex for unique key naming convention: %v", err),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
		return
	}

	if !regex.MatchString(indexName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingUKConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				`Unique key in table "%s" mismatches the naming convention, expect %q but found "%s"`,
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
			Code:   int32(types.NamingUKConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				`Unique key "%s" in table "%s" mismatches the naming convention, its length should be within %d characters`,
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

func (c *namingIndexUKChecker) getTemplateRegexp(tokens map[string]string) (*regexp.Regexp, error) {
	return getTemplateRegexp(c.format, c.templateList, tokens)
}

func (c *namingIndexUKChecker) findIndex(schemaName string, tableName string, indexName string) (string, *catalog.IndexState) {
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
