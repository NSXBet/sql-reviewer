package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ColumnTypeDisallowListRule is the ANTLR-based implementation for checking column type disallow list
type ColumnTypeDisallowListRule struct {
	BaseAntlrRule
	typeRestriction map[string]bool
}

// NewColumnTypeDisallowListRule creates a new ANTLR-based column type disallow list rule
func NewColumnTypeDisallowListRule(
	level types.SQLReviewRuleLevel,
	title string,
	typeRestriction map[string]bool,
) *ColumnTypeDisallowListRule {
	return &ColumnTypeDisallowListRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		typeRestriction: typeRestriction,
	}
}

// Name returns the rule name
func (*ColumnTypeDisallowListRule) Name() string {
	return "ColumnTypeDisallowListRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnTypeDisallowListRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnTypeDisallowListRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnTypeDisallowListRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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
		if tableElement.ColumnDefinition() == nil {
			continue
		}

		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		if tableElement.ColumnDefinition().FieldDefinition() == nil {
			continue
		}
		r.checkFieldDefinition(
			tableName,
			columnName,
			tableElement.ColumnDefinition().FieldDefinition(),
			tableElement.GetStart().GetLine(),
		)
	}
}

func (r *ColumnTypeDisallowListRule) checkFieldDefinition(
	tableName, columnName string,
	ctx mysql.IFieldDefinitionContext,
	line int,
) {
	if ctx.DataType() == nil {
		return
	}
	columnType := mysqlparser.NormalizeMySQLDataType(ctx.DataType(), true /* compact */)
	columnType = strings.ToUpper(columnType)
	if _, exists := r.typeRestriction[columnType]; exists {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DisabledColumnType),
			Title:         r.title,
			Content:       fmt.Sprintf("Disallow column type %s but column `%s`.`%s` is", columnType, tableName, columnName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + line),
		})
	}
}

func (r *ColumnTypeDisallowListRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
	// alter table add column, change column, modify column.
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		switch {
		// add column
		case item.ADD_SYMBOL() != nil:
			switch {
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
				r.checkFieldDefinition(tableName, columnName, item.FieldDefinition(), ctx.GetStart().GetLine())
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil ||
						tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.checkFieldDefinition(
						tableName,
						columnName,
						tableElement.ColumnDefinition().FieldDefinition(),
						ctx.GetStart().GetLine(),
					)
				}
			}
		// modify column
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			columnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition(), ctx.GetStart().GetLine())
		// change column
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition(), ctx.GetStart().GetLine())
		}
	}
}

// ColumnTypeDisallowListAdvisor is the advisor using ANTLR parser for column type disallow list checking
type ColumnTypeDisallowListAdvisor struct{}

// Check performs the ANTLR-based column type disallow list check using payload
func (a *ColumnTypeDisallowListAdvisor) Check(
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

	// Parse the disallowed types from rule payload
	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	typeRestriction := make(map[string]bool)
	for _, typeStr := range payload.List {
		typeStr = strings.TrimSpace(strings.ToUpper(typeStr))
		if typeStr != "" {
			typeRestriction[typeStr] = true
		}
	}

	// Create the rule
	typeDisallowRule := NewColumnTypeDisallowListRule(types.SQLReviewRuleLevel(level), string(rule.Type), typeRestriction)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{typeDisallowRule})

	for _, stmtNode := range root {
		typeDisallowRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
