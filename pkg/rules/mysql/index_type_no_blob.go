package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/pkg/errors"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// IndexTypeNoBlobRule is the ANTLR-based implementation for checking index type no blob
type IndexTypeNoBlobRule struct {
	BaseAntlrRule
	tablesNewColumns tableColumnTypes
	catalog          *types.DatabaseSchemaMetadata
}

// NewIndexTypeNoBlobRule creates a new ANTLR-based index type no blob rule
func NewIndexTypeNoBlobRule(level types.SQLReviewRuleLevel, title string, catalog *types.DatabaseSchemaMetadata) *IndexTypeNoBlobRule {
	return &IndexTypeNoBlobRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		tablesNewColumns: make(tableColumnTypes),
		catalog:          catalog,
	}
}

// Name returns the rule name
func (*IndexTypeNoBlobRule) Name() string {
	return "IndexTypeNoBlobRule"
}

// OnEnter is called when entering a parse tree node
func (r *IndexTypeNoBlobRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeCreateIndex:
		r.checkCreateIndex(ctx.(*mysql.CreateIndexContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*IndexTypeNoBlobRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *IndexTypeNoBlobRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	tableName := NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		switch {
		case tableElement.ColumnDefinition() != nil:
			if tableElement.ColumnDefinition().FieldDefinition() == nil {
				continue
			}
			columnName := NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
			r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
		case tableElement.TableConstraintDef() != nil:
			r.checkConstraintDef(tableName, tableElement.TableConstraintDef())
		}
	}
}

func (r *IndexTypeNoBlobRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

	tableName := NormalizeMySQLTableRef(ctx.TableRef())
	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if alterListItem == nil {
			continue
		}

		switch {
		case alterListItem.ADD_SYMBOL() != nil:
			switch {
			// add column.
			case alterListItem.Identifier() != nil && alterListItem.FieldDefinition() != nil:
				columnName := NormalizeMySQLIdentifier(alterListItem.Identifier())
				r.checkFieldDefinition(tableName, columnName, alterListItem.FieldDefinition())
			// add multi column.
			case alterListItem.OPEN_PAR_SYMBOL() != nil && alterListItem.TableElementList() != nil:
				for _, tableElement := range alterListItem.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					columnName := NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
				}
			// add constraint.
			case alterListItem.TableConstraintDef() != nil:
				r.checkConstraintDef(tableName, alterListItem.TableConstraintDef())
			}
		// modify column
		case alterListItem.MODIFY_SYMBOL() != nil && alterListItem.ColumnInternalRef() != nil:
			columnName := NormalizeMySQLColumnInternalRef(alterListItem.ColumnInternalRef())
			r.checkFieldDefinition(tableName, columnName, alterListItem.FieldDefinition())
		// change column
		case alterListItem.CHANGE_SYMBOL() != nil && alterListItem.ColumnInternalRef() != nil && alterListItem.Identifier() != nil:
			oldColumnName := NormalizeMySQLColumnInternalRef(alterListItem.ColumnInternalRef())
			r.tablesNewColumns.delete(tableName, oldColumnName)
			newColumnName := NormalizeMySQLIdentifier(alterListItem.Identifier())
			r.checkFieldDefinition(tableName, newColumnName, alterListItem.FieldDefinition())
		}
	}
}

func (r *IndexTypeNoBlobRule) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	if ctx.GetType_() == nil {
		return
	}
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserFULLTEXT_SYMBOL, mysql.MySQLParserSPATIAL_SYMBOL, mysql.MySQLParserFOREIGN_SYMBOL:
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil || ctx.CreateIndexTarget().KeyListVariants() == nil {
		return
	}

	tableName := NormalizeMySQLTableRef(ctx.CreateIndexTarget().TableRef())
	columnList := NormalizeKeyListVariants(ctx.CreateIndexTarget().KeyListVariants())
	for _, columnName := range columnList {
		columnType, err := r.getColumnType(tableName, columnName)
		if err != nil {
			continue
		}
		columnType = strings.ToLower(columnType)
		r.addAdvice(tableName, columnName, columnType, ctx.GetStart().GetLine())
	}
}

func (r *IndexTypeNoBlobRule) checkFieldDefinition(tableName, columnName string, ctx mysql.IFieldDefinitionContext) {
	if ctx.DataType() == nil {
		return
	}
	columnType := NormalizeMySQLDataType(ctx.DataType())
	for _, attribute := range ctx.AllColumnAttribute() {
		if attribute == nil || attribute.GetValue() == nil {
			continue
		}
		// the FieldDefinitionContext can only set primary or unique.
		switch attribute.GetValue().GetTokenType() {
		case mysql.MySQLParserPRIMARY_SYMBOL, mysql.MySQLParserUNIQUE_SYMBOL:
			// do nothing
		default:
			continue
		}
		r.addAdvice(tableName, columnName, columnType, ctx.GetStart().GetLine())
	}
	r.tablesNewColumns.set(tableName, columnName, columnType)
}

func (r *IndexTypeNoBlobRule) checkConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
	if ctx.GetType_() == nil {
		return
	}
	var columnList []string
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserINDEX_SYMBOL, mysql.MySQLParserKEY_SYMBOL, mysql.MySQLParserPRIMARY_SYMBOL, mysql.MySQLParserUNIQUE_SYMBOL:
		if ctx.KeyListVariants() == nil {
			return
		}
		columnList = NormalizeKeyListVariants(ctx.KeyListVariants())
	case mysql.MySQLParserFOREIGN_SYMBOL:
		if ctx.KeyList() == nil {
			return
		}
		columnList = NormalizeKeyList(ctx.KeyList())
	default:
		return
	}

	for _, columnName := range columnList {
		columnType, err := r.getColumnType(tableName, columnName)
		if err != nil {
			continue
		}
		columnType = strings.ToLower(columnType)
		r.addAdvice(tableName, columnName, columnType, ctx.GetStart().GetLine())
	}
}

func (r *IndexTypeNoBlobRule) addAdvice(tableName, columnName, columnType string, lineNumber int) {
	if r.isBlob(columnType) {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.IndexTypeNoBlob),
			Title:         r.title,
			Content:       fmt.Sprintf("Columns in index must not be BLOB but `%s`.`%s` is %s", tableName, columnName, columnType),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + lineNumber),
		})
	}
}

func (*IndexTypeNoBlobRule) isBlob(columnType string) bool {
	switch strings.ToLower(columnType) {
	case "blob", "tinyblob", "mediumblob", "longblob":
		return true
	default:
		return false
	}
}

// getColumnType gets the column type string from r.tableColumnTypes or catalog, returns empty string and non-nil error if cannot find the column in given table.
func (r *IndexTypeNoBlobRule) getColumnType(tableName string, columnName string) (string, error) {
	if columnType, ok := r.tablesNewColumns.get(tableName, columnName); ok {
		return columnType, nil
	}
	if r.catalog != nil {
		// Search through schemas for the table
		for _, schema := range r.catalog.Schemas {
			for _, table := range schema.Tables {
				if table.Name == tableName {
					for _, column := range table.Columns {
						if column.Name == columnName {
							return column.Type, nil
						}
					}
				}
			}
		}
	}
	return "", errors.Errorf("cannot find the type of `%s`.`%s`", tableName, columnName)
}

// IndexTypeNoBlobAdvisor is the advisor using ANTLR parser for index type no blob checking
type IndexTypeNoBlobAdvisor struct{}

// Check performs the ANTLR-based index type no blob check
func (a *IndexTypeNoBlobAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule with catalog support
	indexTypeNoBlobRule := NewIndexTypeNoBlobRule(types.SQLReviewRuleLevel(level), string(rule.Type), checkContext.DBSchema)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{indexTypeNoBlobRule})

	for _, stmtNode := range root {
		indexTypeNoBlobRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}