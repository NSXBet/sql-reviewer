package mysql

import (
	"context"
	"fmt"
	"slices"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/pkg/errors"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

const (
	maxDefaultCurrentTimeColumnCount   = 2
	maxOnUpdateCurrentTimeColumnCount = 1
)

type currentTimeTableData struct {
	tableName                string
	defaultCurrentTimeCount  int
	onUpdateCurrentTimeCount int
	line                     int
}

// ColumnCurrentTimeCountLimitRule is the ANTLR-based implementation for checking current time column count limit
type ColumnCurrentTimeCountLimitRule struct {
	BaseAntlrRule
	tableSet map[string]currentTimeTableData
}

// NewColumnCurrentTimeCountLimitRule creates a new ANTLR-based column current time count limit rule
func NewColumnCurrentTimeCountLimitRule(level types.SQLReviewRuleLevel, title string) *ColumnCurrentTimeCountLimitRule {
	return &ColumnCurrentTimeCountLimitRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		tableSet: make(map[string]currentTimeTableData),
	}
}

// Name returns the rule name
func (*ColumnCurrentTimeCountLimitRule) Name() string {
	return "ColumnCurrentTimeCountLimitRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnCurrentTimeCountLimitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnCurrentTimeCountLimitRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnCurrentTimeCountLimitRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableElementList() == nil || ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil || tableElement.ColumnDefinition().FieldDefinition().DataType() == nil {
			continue
		}
		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		r.checkTime(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
	}
}

func (r *ColumnCurrentTimeCountLimitRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
				r.checkTime(tableName, columnName, item.FieldDefinition())
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.checkTime(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
				}
			}
		// change column.
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			// only focus on new column name.
			columnName = mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.checkTime(tableName, columnName, item.FieldDefinition())
		// modify column.
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			columnName = mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			r.checkTime(tableName, columnName, item.FieldDefinition())
		default:
			continue
		}
	}
}

func (r *ColumnCurrentTimeCountLimitRule) checkTime(tableName string, _ string, ctx mysql.IFieldDefinitionContext) {
	if ctx.DataType() == nil {
		return
	}

	switch ctx.DataType().GetType_().GetTokenType() {
	case mysql.MySQLParserDATETIME_SYMBOL, mysql.MySQLParserTIMESTAMP_SYMBOL:
		if r.isDefaultCurrentTime(ctx) {
			table, exists := r.tableSet[tableName]
			if !exists {
				table = currentTimeTableData{
					tableName: tableName,
				}
			}
			table.defaultCurrentTimeCount++
			table.line = r.baseLine + ctx.GetStart().GetLine()
			r.tableSet[tableName] = table
		}
		if r.isOnUpdateCurrentTime(ctx) {
			table, exists := r.tableSet[tableName]
			if !exists {
				table = currentTimeTableData{
					tableName: tableName,
				}
			}
			table.onUpdateCurrentTimeCount++
			table.line = r.baseLine + ctx.GetStart().GetLine()
			r.tableSet[tableName] = table
		}
	}
}

func (*ColumnCurrentTimeCountLimitRule) isDefaultCurrentTime(ctx mysql.IFieldDefinitionContext) bool {
	for _, attr := range ctx.AllColumnAttribute() {
		if attr == nil || attr.GetValue() == nil {
			continue
		}
		if attr.GetValue().GetTokenType() == mysql.MySQLParserDEFAULT_SYMBOL && attr.NOW_SYMBOL() != nil {
			return true
		}
	}
	return false
}

func (*ColumnCurrentTimeCountLimitRule) isOnUpdateCurrentTime(ctx mysql.IFieldDefinitionContext) bool {
	for _, attr := range ctx.AllColumnAttribute() {
		if attr == nil || attr.GetValue() == nil {
			continue
		}
		if attr.GetValue().GetTokenType() == mysql.MySQLParserON_SYMBOL && attr.NOW_SYMBOL() != nil {
			return true
		}
	}
	return false
}

func (r *ColumnCurrentTimeCountLimitRule) generateAdvice() {
	var tableList []currentTimeTableData
	for _, table := range r.tableSet {
		tableList = append(tableList, table)
	}
	slices.SortFunc(tableList, func(a, b currentTimeTableData) int {
		if a.line < b.line {
			return -1
		}
		if a.line > b.line {
			return 1
		}
		return 0
	})
	for _, table := range tableList {
		if table.defaultCurrentTimeCount > maxDefaultCurrentTimeColumnCount {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.DefaultCurrentTimeColumnCountExceedsLimit),
				Title:         r.title,
				Content:       fmt.Sprintf("Table `%s` has %d DEFAULT CURRENT_TIMESTAMP() columns. The count greater than %d.", table.tableName, table.defaultCurrentTimeCount, maxDefaultCurrentTimeColumnCount),
				StartPosition: ConvertANTLRLineToPosition(table.line),
			})
		}
		if table.onUpdateCurrentTimeCount > maxOnUpdateCurrentTimeColumnCount {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.OnUpdateCurrentTimeColumnCountExceedsLimit),
				Title:         r.title,
				Content:       fmt.Sprintf("Table `%s` has %d ON UPDATE CURRENT_TIMESTAMP() columns. The count greater than %d.", table.tableName, table.onUpdateCurrentTimeCount, maxOnUpdateCurrentTimeColumnCount),
				StartPosition: ConvertANTLRLineToPosition(table.line),
			})
		}
	}
}

// ColumnCurrentTimeCountLimitAdvisor is the advisor using ANTLR parser for column current time count limit checking
type ColumnCurrentTimeCountLimitAdvisor struct{}

// Check performs the ANTLR-based column current time count limit check
func (a *ColumnCurrentTimeCountLimitAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule (doesn't need catalog)
	currentTimeRule := NewColumnCurrentTimeCountLimitRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{currentTimeRule})

	for _, stmtNode := range root {
		currentTimeRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	// Generate advice after walking
	currentTimeRule.generateAdvice()

	return checker.GetAdviceList(), nil
}