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

var _ advisor.Advisor = (*NamingIndexPKAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRulePKNaming), &NamingIndexPKAdvisor{})
}

// NamingIndexPKAdvisor is the advisor for primary key naming convention.
type NamingIndexPKAdvisor struct{}

// Check checks the primary key naming convention.
func (*NamingIndexPKAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
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

	checker := &namingIndexPKChecker{
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

type namingIndexPKChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList   []*types.Advice
	level        types.Advice_Status
	title        string
	format       string
	maxLength    int
	templateList []string
	catalog      *catalog.Finder
}

type pkMetaData struct {
	pkName     string
	tableName  string
	schemaName string
	line       int
	metaData   map[string]string
}

// EnterCreatestmt handles CREATE TABLE with PRIMARY KEY constraints
func (c *namingIndexPKChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
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

	// Check table-level constraints
	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			// Check table-level PRIMARY KEY constraint
			if elem.Tableconstraint() != nil {
				constraint := elem.Tableconstraint()
				if pkData := c.getPKMetaDataFromTableConstraint(constraint, tableName, schemaName, constraint.GetStart().GetLine()); pkData != nil {
					c.checkPKName(pkData)
				}
			}
		}
	}
}

// EnterAltertablestmt handles ALTER TABLE ADD CONSTRAINT PRIMARY KEY
func (c *namingIndexPKChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
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

	// Check all alter table commands
	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			// ADD CONSTRAINT
			if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
				constraint := cmd.Tableconstraint()
				if pkData := c.getPKMetaDataFromTableConstraint(constraint, tableName, schemaName, constraint.GetStart().GetLine()); pkData != nil {
					c.checkPKName(pkData)
				}
			}
		}
	}
}

// EnterRenamestmt handles ALTER TABLE RENAME CONSTRAINT and ALTER INDEX RENAME TO
func (c *namingIndexPKChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	allNames := ctx.AllName()

	// Check for ALTER TABLE ... RENAME CONSTRAINT old_name TO new_name
	if ctx.TABLE() != nil && ctx.CONSTRAINT() != nil && ctx.TO() != nil {
		var tableName, schemaName string
		if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
			tableName = extractTableName(ctx.Relation_expr().Qualified_name())
			schemaName = extractSchemaName(ctx.Relation_expr().Qualified_name())
		}

		if len(allNames) >= 2 {
			oldName := pgparser.NormalizePostgreSQLName(allNames[len(allNames)-2])
			newName := pgparser.NormalizePostgreSQLName(allNames[len(allNames)-1])

			// Check if old constraint is a primary key using catalog
			if c.catalog != nil {
				normalizedSchema := schemaName
				if normalizedSchema == "" {
					normalizedSchema = "public"
				}
				_, index := c.catalog.Origin.FindIndex(&catalog.IndexFind{
					SchemaName: normalizedSchema,
					TableName:  tableName,
					IndexName:  oldName,
				})
				if index != nil && index.Primary() {
					metaData := map[string]string{
						advisor.ColumnListTemplateToken: strings.Join(index.ExpressionList(), "_"),
						advisor.TableNameTemplateToken:  tableName,
					}
					pkData := &pkMetaData{
						pkName:     newName,
						tableName:  tableName,
						schemaName: schemaName,
						line:       ctx.GetStart().GetLine(),
						metaData:   metaData,
					}
					c.checkPKName(pkData)
				}
			}
		}
	}

	// Check for ALTER INDEX ... RENAME TO new_name
	if ctx.INDEX() != nil && ctx.TO() != nil {
		var oldIndexName, schemaName string

		if ctx.Qualified_name() != nil {
			oldIndexName = extractTableName(ctx.Qualified_name())
			schemaName = extractSchemaName(ctx.Qualified_name())
		} else if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
			oldIndexName = extractTableName(ctx.Relation_expr().Qualified_name())
			schemaName = extractSchemaName(ctx.Relation_expr().Qualified_name())
		}

		if oldIndexName != "" && len(allNames) > 0 {
			newIndexName := pgparser.NormalizePostgreSQLName(allNames[len(allNames)-1])

			// Check if this index is a primary key using catalog
			if c.catalog != nil {
				normalizedSchema := schemaName
				if normalizedSchema == "" {
					normalizedSchema = "public"
				}
				tableName, index := c.catalog.Origin.FindIndex(&catalog.IndexFind{
					SchemaName: normalizedSchema,
					TableName:  "",
					IndexName:  oldIndexName,
				})
				if index != nil && index.Primary() {
					metaData := map[string]string{
						advisor.ColumnListTemplateToken: strings.Join(index.ExpressionList(), "_"),
						advisor.TableNameTemplateToken:  tableName,
					}
					pkData := &pkMetaData{
						pkName:     newIndexName,
						tableName:  tableName,
						schemaName: normalizedSchema,
						line:       ctx.GetStart().GetLine(),
						metaData:   metaData,
					}
					c.checkPKName(pkData)
				}
			}
		}
	}
}

func (c *namingIndexPKChecker) getPKMetaDataFromTableConstraint(
	constraint parser.ITableconstraintContext,
	tableName, schemaName string,
	line int,
) *pkMetaData {
	if constraint == nil || constraint.Constraintelem() == nil {
		return nil
	}

	elem := constraint.Constraintelem()

	// Check if this is a PRIMARY KEY constraint
	if elem.PRIMARY() == nil || elem.KEY() == nil {
		return nil
	}

	var pkName string
	var columnList []string

	// Extract constraint name
	if constraint.Name() != nil {
		pkName = pgparser.NormalizePostgreSQLName(constraint.Name())
	}

	// Get column list
	if elem.Columnlist() != nil {
		allColumns := elem.Columnlist().AllColumnElem()
		for _, col := range allColumns {
			if col.Colid() != nil {
				columnList = append(columnList, pgparser.NormalizePostgreSQLColid(col.Colid()))
			}
		}
	}

	// PRIMARY KEY USING INDEX
	if elem.Existingindex() != nil && elem.Existingindex().Name() != nil {
		indexName := pgparser.NormalizePostgreSQLName(elem.Existingindex().Name())
		if c.catalog != nil && indexName != "" {
			normalizedSchema := schemaName
			if normalizedSchema == "" {
				normalizedSchema = "public"
			}
			_, index := c.catalog.Origin.FindIndex(&catalog.IndexFind{
				SchemaName: normalizedSchema,
				TableName:  tableName,
				IndexName:  indexName,
			})
			if index != nil {
				columnList = index.ExpressionList()
			}
		}
	}

	if pkName != "" {
		metaData := map[string]string{
			advisor.ColumnListTemplateToken: strings.Join(columnList, "_"),
			advisor.TableNameTemplateToken:  tableName,
		}
		return &pkMetaData{
			pkName:     pkName,
			tableName:  tableName,
			schemaName: schemaName,
			line:       line,
			metaData:   metaData,
		}
	}

	return nil
}

func (c *namingIndexPKChecker) checkPKName(pkData *pkMetaData) {
	regex, err := getTemplateRegexp(c.format, c.templateList, pkData.metaData)
	if err != nil {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(advisor.Internal),
			Title:   "Internal error for primary key naming convention rule",
			Content: fmt.Sprintf("Failed to compile regex: %v", err),
			StartPosition: &types.Position{
				Line: int32(pkData.line),
			},
		})
		return
	}

	if !regex.MatchString(pkData.pkName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingIndexConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				`Primary key in table "%s" mismatches the naming convention, expect %q but found "%s"`,
				pkData.tableName,
				regex,
				pkData.pkName,
			),
			StartPosition: &types.Position{
				Line: int32(pkData.line),
			},
		})
	}

	if c.maxLength > 0 && len(pkData.pkName) > c.maxLength {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingIndexConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				`Primary key "%s" in table "%s" mismatches the naming convention, its length should be within %d characters`,
				pkData.pkName,
				pkData.tableName,
				c.maxLength,
			),
			StartPosition: &types.Position{
				Line: int32(pkData.line),
			},
		})
	}
}
