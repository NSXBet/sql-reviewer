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

// ColumnRequireDefaultRule is the ANTLR-based implementation for checking column default requirement
type ColumnRequireDefaultRule struct {
	BaseAntlrRule
}

// NewColumnRequireDefaultRule creates a new ANTLR-based column require default rule
func NewColumnRequireDefaultRule(level types.SQLReviewRuleLevel, title string) *ColumnRequireDefaultRule {
	return &ColumnRequireDefaultRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnRequireDefaultRule) Name() string {
	return "ColumnRequireDefaultRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnRequireDefaultRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnRequireDefaultRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnRequireDefaultRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	pkColumns := r.getPKColumns(ctx)
	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		if tableElement.ColumnDefinition() == nil {
			continue
		}

		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		if pkColumns[columnName] {
			continue
		}
		if tableElement.ColumnDefinition().FieldDefinition() == nil {
			continue
		}
		r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
	}
}

func (r *ColumnRequireDefaultRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
		// modify column
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			columnName = mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		// change column
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			columnName = mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		}
	}
}

func (r *ColumnRequireDefaultRule) checkFieldDefinition(tableName, columnName string, ctx mysql.IFieldDefinitionContext) {
	if !r.hasDefault(ctx) && r.columnNeedDefault(ctx) {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.ColumnRequireDefault),
			Title:         r.title,
			Content:       fmt.Sprintf("Column `%s`.`%s` doesn't have DEFAULT.", tableName, columnName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (*ColumnRequireDefaultRule) hasDefault(ctx mysql.IFieldDefinitionContext) bool {
	for _, attr := range ctx.AllColumnAttribute() {
		if attr.DEFAULT_SYMBOL() != nil {
			return true
		}
	}
	return false
}

func (*ColumnRequireDefaultRule) getPKColumns(ctx *mysql.CreateTableContext) map[string]bool {
	pkColumn := make(map[string]bool)
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		if tableElement.TableConstraintDef() == nil {
			continue
		}

		if tableElement.TableConstraintDef().GetType_().GetTokenType() != mysql.MySQLParserPRIMARY_SYMBOL {
			continue
		}
		if tableElement.TableConstraintDef().KeyListVariants() == nil {
			continue
		}
		columnList := mysqlparser.NormalizeKeyListVariants(tableElement.TableConstraintDef().KeyListVariants())
		for _, column := range columnList {
			pkColumn[column] = true
		}
	}

	// Also check for inline primary key definitions
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil {
			continue
		}
		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		for _, attr := range tableElement.ColumnDefinition().FieldDefinition().AllColumnAttribute() {
			if attr.PRIMARY_SYMBOL() != nil {
				pkColumn[columnName] = true
				break
			}
		}
	}

	return pkColumn
}

func (*ColumnRequireDefaultRule) columnNeedDefault(ctx mysql.IFieldDefinitionContext) bool {
	if ctx.GENERATED_SYMBOL() != nil {
		return false
	}
	for _, attr := range ctx.AllColumnAttribute() {
		if attr.AUTO_INCREMENT_SYMBOL() != nil || attr.PRIMARY_SYMBOL() != nil {
			return false
		}
	}

	if ctx.DataType() == nil {
		return false
	}

	switch ctx.DataType().GetType_().GetTokenType() {
	case mysql.MySQLParserBLOB_SYMBOL,
		mysql.MySQLParserTINYBLOB_SYMBOL,
		mysql.MySQLParserMEDIUMBLOB_SYMBOL,
		mysql.MySQLParserLONGBLOB_SYMBOL,
		mysql.MySQLParserJSON_SYMBOL,
		mysql.MySQLParserTINYTEXT_SYMBOL,
		mysql.MySQLParserTEXT_SYMBOL,
		mysql.MySQLParserMEDIUMTEXT_SYMBOL,
		mysql.MySQLParserLONGTEXT_SYMBOL,
		mysql.MySQLParserLONG_SYMBOL,
		mysql.MySQLParserSERIAL_SYMBOL,
		mysql.MySQLParserGEOMETRY_SYMBOL,
		mysql.MySQLParserGEOMETRYCOLLECTION_SYMBOL,
		mysql.MySQLParserPOINT_SYMBOL,
		mysql.MySQLParserMULTIPOINT_SYMBOL,
		mysql.MySQLParserLINESTRING_SYMBOL,
		mysql.MySQLParserMULTILINESTRING_SYMBOL,
		mysql.MySQLParserPOLYGON_SYMBOL,
		mysql.MySQLParserMULTIPOLYGON_SYMBOL:
		return false
	}
	return true
}

// ColumnRequireDefaultAdvisor is the advisor using ANTLR parser for column require default checking
type ColumnRequireDefaultAdvisor struct{}

// Check performs the ANTLR-based column require default check
func (a *ColumnRequireDefaultAdvisor) Check(
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
	defaultRule := NewColumnRequireDefaultRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{defaultRule})

	for _, stmtNode := range root {
		defaultRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
