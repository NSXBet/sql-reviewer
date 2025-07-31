package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ColumnAutoIncrementMustUnsignedRule is the ANTLR-based implementation for checking unsigned auto-increment column
type ColumnAutoIncrementMustUnsignedRule struct {
	BaseAntlrRule
}

// NewColumnAutoIncrementMustUnsignedRule creates a new ANTLR-based auto-increment column unsigned rule
func NewColumnAutoIncrementMustUnsignedRule(level types.SQLReviewRuleLevel, title string) *ColumnAutoIncrementMustUnsignedRule {
	return &ColumnAutoIncrementMustUnsignedRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnAutoIncrementMustUnsignedRule) Name() string {
	return "ColumnAutoIncrementMustUnsignedRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnAutoIncrementMustUnsignedRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnAutoIncrementMustUnsignedRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnAutoIncrementMustUnsignedRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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

func (r *ColumnAutoIncrementMustUnsignedRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		var columnName string
		switch {
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
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			columnName = mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			columnName = mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		}
	}
}

func (r *ColumnAutoIncrementMustUnsignedRule) checkFieldDefinition(
	tableName, columnName string,
	ctx mysql.IFieldDefinitionContext,
) {
	if !r.isAutoIncrementColumnIsUnsigned(ctx) {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.AutoIncrementColumnSigned),
			Title:         r.title,
			Content:       fmt.Sprintf("Auto-increment column `%s`.`%s` is not UNSIGNED type", tableName, columnName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *ColumnAutoIncrementMustUnsignedRule) isAutoIncrementColumnIsUnsigned(ctx mysql.IFieldDefinitionContext) bool {
	if r.isAutoIncrementColumn(ctx) && !r.isUnsigned(ctx) {
		return false
	}
	return true
}

func (*ColumnAutoIncrementMustUnsignedRule) isAutoIncrementColumn(ctx mysql.IFieldDefinitionContext) bool {
	for _, attr := range ctx.AllColumnAttribute() {
		if attr.AUTO_INCREMENT_SYMBOL() != nil {
			return true
		}
	}
	return false
}

func (*ColumnAutoIncrementMustUnsignedRule) isUnsigned(ctx mysql.IFieldDefinitionContext) bool {
	if ctx.DataType() == nil {
		return false
	}

	// Check if UNSIGNED is specified in the data type
	dataTypeText := ctx.DataType().GetParser().GetTokenStream().GetTextFromRuleContext(ctx.DataType())
	upperText := strings.ToUpper(dataTypeText)

	// UNSIGNED is explicitly specified or ZEROFILL (which implies UNSIGNED)
	return strings.Contains(upperText, "UNSIGNED") || strings.Contains(upperText, "ZEROFILL")
}

// ColumnAutoIncrementMustUnsignedAdvisor is the advisor using ANTLR parser for auto-increment column unsigned checking
type ColumnAutoIncrementMustUnsignedAdvisor struct{}

// Check performs the ANTLR-based auto-increment column unsigned check
func (a *ColumnAutoIncrementMustUnsignedAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule (doesn't need catalog)
	unsignedRule := NewColumnAutoIncrementMustUnsignedRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{unsignedRule})

	for _, stmtNode := range root {
		unsignedRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
