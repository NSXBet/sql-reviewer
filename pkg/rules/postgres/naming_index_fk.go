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

var _ advisor.Advisor = (*NamingIndexFKAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleFKNaming), &NamingIndexFKAdvisor{})
}

// NamingIndexFKAdvisor is the advisor for foreign key naming convention.
type NamingIndexFKAdvisor struct{}

// Check checks the foreign key naming convention.
func (*NamingIndexFKAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	format, templateList, maxLength, err := advisor.UnmarshalNamingRulePayloadAsTemplate(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &namingIndexFKChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		format:                       format,
		maxLength:                    maxLength,
		templateList:                 templateList,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type namingIndexFKChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList   []*types.Advice
	level        types.Advice_Status
	title        string
	format       string
	maxLength    int
	templateList []string
}

type fkMetaData struct {
	fkName    string
	tableName string
	line      int
	metaData  map[string]string
}

// EnterCreatestmt handles CREATE TABLE with FOREIGN KEY constraints
func (c *namingIndexFKChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Opttableelementlist() == nil {
		return
	}

	allQualifiedNames := ctx.AllQualified_name()
	if len(allQualifiedNames) == 0 {
		return
	}

	tableName := extractTableName(allQualifiedNames[0])

	// Check table-level and column-level constraints
	if ctx.Opttableelementlist().Tableelementlist() != nil {
		for _, element := range ctx.Opttableelementlist().Tableelementlist().AllTableelement() {
			// Check table-level constraints
			if element.Tableconstraint() != nil {
				metadata := c.extractFKMetadata(element.Tableconstraint(), tableName, ctx.GetStart().GetLine())
				if metadata != nil {
					c.checkFKMetadata(metadata)
				}
			}

			// Check column-level constraints
			if element.ColumnDef() != nil {
				columnDef := element.ColumnDef()
				if columnDef.Colquallist() != nil {
					allQuals := columnDef.Colquallist().AllColconstraint()
					for _, qual := range allQuals {
						metadata := c.extractColumnFKMetadata(qual, tableName, columnDef, columnDef.GetStart().GetLine())
						if metadata != nil {
							c.checkFKMetadata(metadata)
						}
					}
				}
			}
		}
	}
}

// EnterAltertablestmt handles ALTER TABLE ADD CONSTRAINT FOREIGN KEY
func (c *namingIndexFKChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil {
		return
	}

	tableName := extractTableName(ctx.Relation_expr().Qualified_name())

	if ctx.Alter_table_cmds() == nil {
		return
	}

	allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
	for _, cmd := range allCmds {
		// Check for ADD + Tableconstraint
		if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
			metadata := c.extractFKMetadata(cmd.Tableconstraint(), tableName, cmd.GetStart().GetLine())
			if metadata != nil {
				c.checkFKMetadata(metadata)
			}
		}

		// Check for ADD COLUMN with inline foreign key constraint
		if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
			columnDef := cmd.ColumnDef()
			if columnDef.Colquallist() != nil {
				allQuals := columnDef.Colquallist().AllColconstraint()
				for _, qual := range allQuals {
					metadata := c.extractColumnFKMetadata(qual, tableName, columnDef, columnDef.GetStart().GetLine())
					if metadata != nil {
						c.checkFKMetadata(metadata)
					}
				}
			}
		}
	}
}

func (c *namingIndexFKChecker) extractFKMetadata(
	constraint parser.ITableconstraintContext,
	tableName string,
	line int,
) *fkMetaData {
	if constraint.Constraintelem() == nil {
		return nil
	}

	elem := constraint.Constraintelem()

	// Check if this is a FOREIGN KEY constraint
	if elem.FOREIGN() == nil || elem.KEY() == nil {
		return nil
	}

	// Extract constraint name
	constraintName := ""
	if constraint.Name() != nil {
		constraintName = pgparser.NormalizePostgreSQLName(constraint.Name())
	}

	// Only check if we have a constraint name
	if constraintName == "" {
		return nil
	}

	// Extract referencing columns
	var referencingColumns []string
	if elem.Columnlist() != nil {
		for _, colElem := range elem.Columnlist().AllColumnElem() {
			if colElem.Colid() != nil {
				referencingColumns = append(referencingColumns, pgparser.NormalizePostgreSQLColid(colElem.Colid()))
			}
		}
	}

	// Extract referenced table and columns
	var referencedTable string
	var referencedColumns []string
	if elem.Qualified_name() != nil {
		referencedTable = extractTableName(elem.Qualified_name())
	}

	if elem.Opt_column_list() != nil && elem.Opt_column_list().Columnlist() != nil {
		for _, colElem := range elem.Opt_column_list().Columnlist().AllColumnElem() {
			if colElem.Colid() != nil {
				referencedColumns = append(referencedColumns, pgparser.NormalizePostgreSQLColid(colElem.Colid()))
			}
		}
	}

	metaData := map[string]string{
		advisor.ReferencingTableNameTemplateToken:  tableName,
		advisor.ReferencingColumnNameTemplateToken: strings.Join(referencingColumns, "_"),
		advisor.ReferencedTableNameTemplateToken:   referencedTable,
		advisor.ReferencedColumnNameTemplateToken:  strings.Join(referencedColumns, "_"),
	}

	return &fkMetaData{
		fkName:    constraintName,
		tableName: tableName,
		line:      line,
		metaData:  metaData,
	}
}

func (c *namingIndexFKChecker) extractColumnFKMetadata(
	qual parser.IColconstraintContext,
	tableName string,
	columnDef parser.IColumnDefContext,
	line int,
) *fkMetaData {
	if qual.Colconstraintelem() == nil {
		return nil
	}

	elem := qual.Colconstraintelem()

	// Check if this is a REFERENCES constraint (inline foreign key)
	if elem.REFERENCES() == nil {
		return nil
	}

	// Extract constraint name
	constraintName := ""
	if qual.Name() != nil {
		constraintName = pgparser.NormalizePostgreSQLName(qual.Name())
	}

	// Only check if we have a constraint name
	if constraintName == "" {
		return nil
	}

	// Extract referencing column (the column being defined)
	var referencingColumn string
	if columnDef.Colid() != nil {
		referencingColumn = pgparser.NormalizePostgreSQLColid(columnDef.Colid())
	}

	// Extract referenced table and columns
	var referencedTable string
	var referencedColumns []string
	if elem.Qualified_name() != nil {
		referencedTable = extractTableName(elem.Qualified_name())
	}

	if elem.Opt_column_list() != nil && elem.Opt_column_list().Columnlist() != nil {
		for _, colElem := range elem.Opt_column_list().Columnlist().AllColumnElem() {
			if colElem.Colid() != nil {
				referencedColumns = append(referencedColumns, pgparser.NormalizePostgreSQLColid(colElem.Colid()))
			}
		}
	}

	metaData := map[string]string{
		advisor.ReferencingTableNameTemplateToken:  tableName,
		advisor.ReferencingColumnNameTemplateToken: referencingColumn,
		advisor.ReferencedTableNameTemplateToken:   referencedTable,
		advisor.ReferencedColumnNameTemplateToken:  strings.Join(referencedColumns, "_"),
	}

	return &fkMetaData{
		fkName:    constraintName,
		tableName: tableName,
		line:      line,
		metaData:  metaData,
	}
}

func (c *namingIndexFKChecker) checkFKMetadata(fkData *fkMetaData) {
	regex, err := c.getTemplateRegexp(fkData.metaData)
	if err != nil {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.Internal),
			Title:   "Internal error for foreign key naming convention rule",
			Content: fmt.Sprintf("Failed to compile regex: %s", err.Error()),
			StartPosition: &types.Position{
				Line: int32(fkData.line),
			},
		})
		return
	}

	if !regex.MatchString(fkData.fkName) {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingFKConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				`Foreign key in table "%s" mismatches the naming convention, expect %q but found "%s"`,
				fkData.tableName,
				regex,
				fkData.fkName,
			),
			StartPosition: &types.Position{
				Line: int32(fkData.line),
			},
		})
	}

	if c.maxLength > 0 && len(fkData.fkName) > c.maxLength {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.NamingFKConventionMismatch),
			Title:  c.title,
			Content: fmt.Sprintf(
				`Foreign key "%s" in table "%s" mismatches the naming convention, its length should be within %d characters`,
				fkData.fkName,
				fkData.tableName,
				c.maxLength,
			),
			StartPosition: &types.Position{
				Line: int32(fkData.line),
			},
		})
	}
}

func (c *namingIndexFKChecker) getTemplateRegexp(tokens map[string]string) (*regexp.Regexp, error) {
	return getTemplateRegexp(c.format, c.templateList, tokens)
}
