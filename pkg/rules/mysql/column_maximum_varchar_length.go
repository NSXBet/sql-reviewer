package mysql

import (
	"context"
	"fmt"
	"strconv"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ColumnMaximumVarcharLengthRule is the ANTLR-based implementation for checking maximum varchar length
type ColumnMaximumVarcharLengthRule struct {
	BaseAntlrRule
	maximum int
}

// NewColumnMaximumVarcharLengthRule creates a new ANTLR-based column maximum varchar length rule
func NewColumnMaximumVarcharLengthRule(level types.SQLReviewRuleLevel, title string, maximum int) *ColumnMaximumVarcharLengthRule {
	return &ColumnMaximumVarcharLengthRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		maximum: maximum,
	}
}

// Name returns the rule name
func (*ColumnMaximumVarcharLengthRule) Name() string {
	return "ColumnMaximumVarcharLengthRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnMaximumVarcharLengthRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnMaximumVarcharLengthRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnMaximumVarcharLengthRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableElementList() == nil || ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	if tableName == "" {
		return
	}
	
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement.ColumnDefinition() == nil {
			continue
		}
		if tableElement.ColumnDefinition().FieldDefinition() == nil {
			continue
		}
		if tableElement.ColumnDefinition().FieldDefinition().DataType() == nil {
			continue
		}
		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		length := r.getVarcharLength(tableElement.ColumnDefinition().FieldDefinition().DataType())
		if r.maximum > 0 && length > r.maximum {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.VarcharLengthExceedsLimit),
				Title:         r.title,
				Content:       fmt.Sprintf("The length of the VARCHAR column `%s.%s` is bigger than %d", tableName, columnName, r.maximum),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + tableElement.GetStart().GetLine()),
			})
		}
	}
}

func (r *ColumnMaximumVarcharLengthRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

		var columnList []string
		varcharLengthMap := make(map[string]int)
		switch {
		// add column.
		case item.ADD_SYMBOL() != nil:
			switch {
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				if item.FieldDefinition().DataType() == nil {
					continue
				}
				columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
				length := r.getVarcharLength(item.FieldDefinition().DataType())
				varcharLengthMap[columnName] = length
				columnList = append(columnList, columnName)
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil {
						continue
					}
					if tableElement.ColumnDefinition().ColumnName() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					if tableElement.ColumnDefinition().FieldDefinition().DataType() == nil {
						continue
					}
					_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					length := r.getVarcharLength(tableElement.ColumnDefinition().FieldDefinition().DataType())
					varcharLengthMap[columnName] = length
					columnList = append(columnList, columnName)
				}
			}
		// change column.
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			length := r.getVarcharLength(item.FieldDefinition().DataType())
			varcharLengthMap[columnName] = length
			columnList = append(columnList, columnName)
		// modify column.
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			length := r.getVarcharLength(item.FieldDefinition().DataType())
			columnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			varcharLengthMap[columnName] = length
			columnList = append(columnList, columnName)
		default:
			continue
		}
		
		for _, columnName := range columnList {
			if length, ok := varcharLengthMap[columnName]; ok && r.maximum > 0 && length > r.maximum {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.VarcharLengthExceedsLimit),
					Title:         r.title,
					Content:       fmt.Sprintf("The length of the VARCHAR column `%s.%s` is bigger than %d", tableName, columnName, r.maximum),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
				})
			}
		}
	}
}

func (*ColumnMaximumVarcharLengthRule) getVarcharLength(ctx mysql.IDataTypeContext) int {
	if ctx.GetType_() == nil {
		return 0
	}

	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserVARCHAR_SYMBOL:
		if ctx.FieldLength() == nil || ctx.FieldLength().Real_ulonglong_number() == nil {
			return 1
		}
		lengthStr := ctx.FieldLength().Real_ulonglong_number().GetText()
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return 0
		}
		return length
	default:
		return 0
	}
}

// ColumnMaximumVarcharLengthAdvisor is the advisor using ANTLR parser for column maximum varchar length checking
type ColumnMaximumVarcharLengthAdvisor struct{}

// Check performs the ANTLR-based column maximum varchar length check using payload
func (a *ColumnMaximumVarcharLengthAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Parse the maximum length from rule payload
	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	maximum := int(payload.Number)

	// Create the rule
	varcharLengthRule := NewColumnMaximumVarcharLengthRule(types.SQLReviewRuleLevel(level), string(rule.Type), maximum)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{varcharLengthRule})

	for _, stmtNode := range root {
		varcharLengthRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}