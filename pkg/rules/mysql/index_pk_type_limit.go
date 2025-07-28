package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"github.com/pkg/errors"
)

// IndexPkTypeLimitRule is the ANTLR-based implementation for checking primary key column types
type IndexPkTypeLimitRule struct {
	BaseAntlrRule
	line             map[string]int
	catalog          *catalog.Finder
	tablesNewColumns tableColumnTypes
}

// NewIndexPkTypeLimitRule creates a new ANTLR-based index PK type limit rule
func NewIndexPkTypeLimitRule(level types.SQLReviewRuleLevel, title string, catalog *catalog.Finder) *IndexPkTypeLimitRule {
	return &IndexPkTypeLimitRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		line:             make(map[string]int),
		catalog:          catalog,
		tablesNewColumns: make(tableColumnTypes),
	}
}

// Name returns the rule name
func (*IndexPkTypeLimitRule) Name() string {
	return "IndexPkTypeLimitRule"
}

// OnEnter is called when entering a parse tree node
func (r *IndexPkTypeLimitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*IndexPkTypeLimitRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *IndexPkTypeLimitRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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

func (r *IndexPkTypeLimitRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

func (r *IndexPkTypeLimitRule) checkFieldDefinition(tableName, columnName string, ctx mysql.IFieldDefinitionContext) {
	if ctx.DataType() == nil {
		return
	}
	columnType := r.getIntOrBigIntStr(ctx.DataType())
	for _, attribute := range ctx.AllColumnAttribute() {
		if attribute.PRIMARY_SYMBOL() != nil {
			r.addAdvice(tableName, columnName, columnType, r.baseLine+ctx.GetStart().GetLine())
		}
	}
	r.tablesNewColumns.set(tableName, columnName, columnType)
}

func (r *IndexPkTypeLimitRule) checkConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
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
		r.addAdvice(tableName, columnName, columnType, r.baseLine+ctx.GetStart().GetLine())
	}
}

func (r *IndexPkTypeLimitRule) addAdvice(tableName, columnName, columnType string, lineNumber int) {
	if !strings.EqualFold(columnType, "INT") && !strings.EqualFold(columnType, "BIGINT") {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.IndexPKType),
			Title:  r.title,
			Content: fmt.Sprintf(
				"Columns in primary key must be INT/BIGINT but `%s`.`%s` is %s",
				tableName,
				columnName,
				columnType,
			),
			StartPosition: ConvertANTLRLineToPosition(lineNumber),
		})
	}
}

// getPKColumnType gets the column type string from r.tablesNewColumns or catalog, returns empty string and non-nil error if cannot find the column in given table.
func (r *IndexPkTypeLimitRule) getPKColumnType(tableName string, columnName string) (string, error) {
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

// getIntOrBigIntStr returns the type string of tp.
func (*IndexPkTypeLimitRule) getIntOrBigIntStr(ctx mysql.IDataTypeContext) string {
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserINT_SYMBOL:
		return "INT"
	case mysql.MySQLParserBIGINT_SYMBOL:
		return "BIGINT"
	}
	return strings.ToLower(ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx))
}

// IndexPkTypeLimitAdvisor is the advisor using ANTLR parser for index PK type limit checking
type IndexPkTypeLimitAdvisor struct{}

// Check performs the ANTLR-based index PK type limit check
func (a *IndexPkTypeLimitAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Get catalog finder
	var catalogFinder *catalog.Finder
	if checkContext.Catalog != nil {
		catalogFinder = checkContext.Catalog.GetFinder()
	}

	// Create the rule with catalog
	pkTypeRule := NewIndexPkTypeLimitRule(types.SQLReviewRuleLevel(level), string(rule.Type), catalogFinder)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{pkTypeRule})

	for _, stmtNode := range root {
		pkTypeRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
