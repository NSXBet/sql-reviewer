package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"github.com/pkg/errors"
)

// ColumnAutoIncrementMustIntegerRule is the ANTLR-based implementation for checking auto-increment column type
type ColumnAutoIncrementMustIntegerRule struct {
	BaseAntlrRule
}

// NewColumnAutoIncrementMustIntegerRule creates a new ANTLR-based auto-increment column type rule
func NewColumnAutoIncrementMustIntegerRule(level types.SQLReviewRuleLevel, title string) *ColumnAutoIncrementMustIntegerRule {
	return &ColumnAutoIncrementMustIntegerRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnAutoIncrementMustIntegerRule) Name() string {
	return "ColumnAutoIncrementMustIntegerRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnAutoIncrementMustIntegerRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnAutoIncrementMustIntegerRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnAutoIncrementMustIntegerRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableElementList() == nil || ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil ||
			tableElement.ColumnDefinition().FieldDefinition().DataType() == nil {
			continue
		}
		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
	}
}

func (r *ColumnAutoIncrementMustIntegerRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	if tableName == "" {
		return
	}
	// alter table add column, change column, modify column.
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		var columnName string
		switch {
		// add column
		case item.ADD_SYMBOL() != nil:
			switch {
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
				r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil ||
						tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
				}
			}
		// change column.
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			columnName = mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		// modify column.
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			columnName = mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		}
	}
}

func (r *ColumnAutoIncrementMustIntegerRule) checkFieldDefinition(
	tableName, columnName string,
	ctx mysql.IFieldDefinitionContext,
) {
	if !r.isAutoIncrementColumnIsInteger(ctx) {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.AutoIncrementColumnNotInteger),
			Title:         r.title,
			Content:       fmt.Sprintf("Auto-increment column `%s`.`%s` requires integer type", tableName, columnName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *ColumnAutoIncrementMustIntegerRule) isAutoIncrementColumnIsInteger(ctx mysql.IFieldDefinitionContext) bool {
	if r.isAutoIncrementColumn(ctx) && !r.isIntegerType(ctx.DataType()) {
		return false
	}
	return true
}

func (*ColumnAutoIncrementMustIntegerRule) isAutoIncrementColumn(ctx mysql.IFieldDefinitionContext) bool {
	for _, attr := range ctx.AllColumnAttribute() {
		if attr.AUTO_INCREMENT_SYMBOL() != nil {
			return true
		}
	}
	return false
}

func (*ColumnAutoIncrementMustIntegerRule) isIntegerType(ctx mysql.IDataTypeContext) bool {
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserINT_SYMBOL,
		mysql.MySQLParserTINYINT_SYMBOL,
		mysql.MySQLParserSMALLINT_SYMBOL,
		mysql.MySQLParserMEDIUMINT_SYMBOL,
		mysql.MySQLParserBIGINT_SYMBOL:
		return true
	default:
		return false
	}
}

// ColumnAutoIncrementMustIntegerAdvisor is the advisor using ANTLR parser for auto-increment column type checking
type ColumnAutoIncrementMustIntegerAdvisor struct{}

// Check performs the ANTLR-based auto-increment column type check
func (a *ColumnAutoIncrementMustIntegerAdvisor) Check(
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

	// Create the rule (doesn't need catalog)
	autoIncrementRule := NewColumnAutoIncrementMustIntegerRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{autoIncrementRule})

	for _, stmtNode := range root {
		autoIncrementRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
