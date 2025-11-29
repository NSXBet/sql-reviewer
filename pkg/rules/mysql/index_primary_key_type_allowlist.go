package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
	"github.com/pkg/errors"
)

// IndexPrimaryKeyTypeAllowlistRule is the ANTLR-based implementation for checking primary key type allowlist
type IndexPrimaryKeyTypeAllowlistRule struct {
	BaseAntlrRule
	allowlist        map[string]bool
	catalog          *catalog.Finder
	tablesNewColumns tableColumnTypes
}

// NewIndexPrimaryKeyTypeAllowlistRule creates a new ANTLR-based index primary key type allowlist rule
func NewIndexPrimaryKeyTypeAllowlistRule(
	level types.SQLReviewRuleLevel,
	title string,
	allowlist map[string]bool,
	catalog *catalog.Finder,
) *IndexPrimaryKeyTypeAllowlistRule {
	return &IndexPrimaryKeyTypeAllowlistRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		allowlist:        allowlist,
		catalog:          catalog,
		tablesNewColumns: make(tableColumnTypes),
	}
}

// Name returns the rule name
func (*IndexPrimaryKeyTypeAllowlistRule) Name() string {
	return "IndexPrimaryKeyTypeAllowlistRule"
}

// OnEnter is called when entering a parse tree node
func (r *IndexPrimaryKeyTypeAllowlistRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*IndexPrimaryKeyTypeAllowlistRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *IndexPrimaryKeyTypeAllowlistRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		switch {
		case tableElement.ColumnDefinition() != nil:
			if tableElement.ColumnDefinition().FieldDefinition() == nil {
				continue
			}
			_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
			r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
		case tableElement.TableConstraintDef() != nil:
			r.checkConstraintDef(tableName, tableElement.TableConstraintDef())
		}
	}
}

func (r *IndexPrimaryKeyTypeAllowlistRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}
	if ctx.TableRef() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if alterListItem == nil {
			continue
		}

		switch {
		// add column
		case alterListItem.ADD_SYMBOL() != nil && alterListItem.Identifier() != nil:
			switch {
			case alterListItem.Identifier() != nil && alterListItem.FieldDefinition() != nil:
				columnName := mysqlparser.NormalizeMySQLIdentifier(alterListItem.Identifier())
				r.checkFieldDefinition(tableName, columnName, alterListItem.FieldDefinition())
			case alterListItem.OPEN_PAR_SYMBOL() != nil && alterListItem.TableElementList() != nil:
				for _, tableElement := range alterListItem.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil ||
						tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
				}
			}
		// modify column
		case alterListItem.MODIFY_SYMBOL() != nil && alterListItem.ColumnInternalRef() != nil:
			columnName := mysqlparser.NormalizeMySQLColumnInternalRef(alterListItem.ColumnInternalRef())
			r.checkFieldDefinition(tableName, columnName, alterListItem.FieldDefinition())
		// change column
		case alterListItem.CHANGE_SYMBOL() != nil && alterListItem.ColumnInternalRef() != nil && alterListItem.Identifier() != nil:
			oldColumnName := mysqlparser.NormalizeMySQLColumnInternalRef(alterListItem.ColumnInternalRef())
			r.tablesNewColumns.delete(tableName, oldColumnName)
			newColumnName := mysqlparser.NormalizeMySQLIdentifier(alterListItem.Identifier())
			r.checkFieldDefinition(tableName, newColumnName, alterListItem.FieldDefinition())
		// add constraint.
		case alterListItem.ADD_SYMBOL() != nil && alterListItem.TableConstraintDef() != nil:
			r.checkConstraintDef(tableName, alterListItem.TableConstraintDef())
		}
	}
}

func (r *IndexPrimaryKeyTypeAllowlistRule) checkFieldDefinition(tableName, columnName string, ctx mysql.IFieldDefinitionContext) {
	if ctx.DataType() == nil {
		return
	}
	columnType := mysqlparser.NormalizeMySQLDataType(ctx.DataType(), true /* compact */)
	for _, attribute := range ctx.AllColumnAttribute() {
		if attribute.PRIMARY_SYMBOL() != nil {
			if _, exists := r.allowlist[columnType]; !exists {
				r.AddAdvice(&types.Advice{
					Status: types.Advice_Status(r.level),
					Code:   int32(types.IndexPKType),
					Title:  r.title,
					Content: fmt.Sprintf(
						"The column `%s` in table `%s` is one of the primary key, but its type \"%s\" is not in allowlist",
						columnName,
						tableName,
						columnType,
					),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
				})
			}
		}
	}
	r.tablesNewColumns.set(tableName, columnName, columnType)
}

func (r *IndexPrimaryKeyTypeAllowlistRule) checkConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
	if ctx.GetType_().GetTokenType() != mysql.MySQLParserPRIMARY_SYMBOL {
		return
	}
	if ctx.KeyListVariants() == nil {
		return
	}
	columnList := mysqlparser.NormalizeKeyListVariants(ctx.KeyListVariants())

	for _, columnName := range columnList {
		columnType, err := r.getPKColumnType(tableName, columnName)
		if err != nil {
			continue
		}
		columnType = strings.ToLower(columnType)
		if _, exists := r.allowlist[columnType]; !exists {
			r.AddAdvice(&types.Advice{
				Status: types.Advice_Status(r.level),
				Code:   int32(types.IndexPKType),
				Title:  r.title,
				Content: fmt.Sprintf(
					"The column `%s` in table `%s` is one of the primary key, but its type \"%s\" is not in allowlist",
					columnName,
					tableName,
					columnType,
				),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// getPKColumnType gets the column type string from r.tablesNewColumns or catalog, returns empty string and non-nil error if cannot find the column in given table.
func (r *IndexPrimaryKeyTypeAllowlistRule) getPKColumnType(tableName string, columnName string) (string, error) {
	if columnType, ok := r.tablesNewColumns.get(tableName, columnName); ok {
		return columnType, nil
	}

	if r.catalog != nil {
		column := r.catalog.Origin.FindColumn(&catalog.ColumnFind{
			TableName:  tableName,
			ColumnName: columnName,
		})
		if column != nil {
			return column.Type(), nil
		}
	}

	return "", errors.Errorf("cannot find the type of `%s`.`%s`", tableName, columnName)
}

// IndexPrimaryKeyTypeAllowlistAdvisor is the advisor using ANTLR parser for index primary key type allowlist checking
type IndexPrimaryKeyTypeAllowlistAdvisor struct{}

// Check performs the ANTLR-based index primary key type allowlist check using payload
func (a *IndexPrimaryKeyTypeAllowlistAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.Context,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Parse the allowlist from rule payload
	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	allowlist := make(map[string]bool)
	for _, typeStr := range payload.List {
		typeStr = strings.TrimSpace(strings.ToLower(typeStr))
		if typeStr != "" {
			allowlist[typeStr] = true
		}
	}

	// Get catalog finder (nil is acceptable for this rule)
	catalogFinder := getCatalogFinder(checkContext)

	// Create the rule with allowlist and catalog
	pkTypeAllowlistRule := NewIndexPrimaryKeyTypeAllowlistRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
		allowlist,
		catalogFinder,
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{pkTypeAllowlistRule})

	for _, stmtNode := range root {
		pkTypeAllowlistRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
