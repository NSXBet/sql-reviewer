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

// ColumnMaximumCharacterLengthRule is the ANTLR-based implementation for checking maximum character length
type ColumnMaximumCharacterLengthRule struct {
	BaseAntlrRule
	maximum int
}

// NewColumnMaximumCharacterLengthRule creates a new ANTLR-based column maximum character length rule
func NewColumnMaximumCharacterLengthRule(level types.SQLReviewRuleLevel, title string, maximum int) *ColumnMaximumCharacterLengthRule {
	return &ColumnMaximumCharacterLengthRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		maximum: maximum,
	}
}

// Name returns the rule name
func (*ColumnMaximumCharacterLengthRule) Name() string {
	return "ColumnMaximumCharacterLengthRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnMaximumCharacterLengthRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnMaximumCharacterLengthRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnMaximumCharacterLengthRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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
		charLength := r.getCharLength(tableElement.ColumnDefinition().FieldDefinition().DataType())
		if r.maximum > 0 && charLength > r.maximum {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.CharLengthExceedsLimit),
				Title:         r.title,
				Content:       fmt.Sprintf("The length of the CHAR column `%s.%s` is bigger than %d, please use VARCHAR instead", tableName, columnName, r.maximum),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + tableElement.GetStart().GetLine()),
			})
		}
	}
}

func (r *ColumnMaximumCharacterLengthRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
		charLengthMap := make(map[string]int)
		switch {
		// add column.
		case item.ADD_SYMBOL() != nil:
			switch {
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				if item.FieldDefinition().DataType() == nil {
					continue
				}
				columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
				charLength := r.getCharLength(item.FieldDefinition().DataType())
				charLengthMap[columnName] = charLength
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
					charLength := r.getCharLength(tableElement.ColumnDefinition().FieldDefinition().DataType())
					charLengthMap[columnName] = charLength
					columnList = append(columnList, columnName)
				}
			}
		// change column.
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			charLength := r.getCharLength(item.FieldDefinition().DataType())
			charLengthMap[columnName] = charLength
			columnList = append(columnList, columnName)
		// modify column.
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			charLength := r.getCharLength(item.FieldDefinition().DataType())
			columnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			charLengthMap[columnName] = charLength
			columnList = append(columnList, columnName)
		default:
			continue
		}
		
		for _, columnName := range columnList {
			if charLength, ok := charLengthMap[columnName]; ok && r.maximum > 0 && charLength > r.maximum {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.CharLengthExceedsLimit),
					Title:         r.title,
					Content:       fmt.Sprintf("The length of the CHAR column `%s.%s` is bigger than %d, please use VARCHAR instead", tableName, columnName, r.maximum),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
				})
			}
		}
	}
}

func (*ColumnMaximumCharacterLengthRule) getCharLength(ctx mysql.IDataTypeContext) int {
	if ctx.GetType_() == nil {
		return 0
	}
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserCHAR_SYMBOL:
		// for mysql: create table tt(a char) == create table tt(a char(1));
		if ctx.FieldLength() == nil || ctx.FieldLength().Real_ulonglong_number() == nil {
			return 1
		}
		charLengthStr := ctx.FieldLength().Real_ulonglong_number().GetText()
		charLengthInt, err := strconv.Atoi(charLengthStr)
		if err != nil {
			return 0
		}
		return charLengthInt
	default:
		return 0
	}
}

// ColumnMaximumCharacterLengthAdvisor is the advisor using ANTLR parser for column maximum character length checking
type ColumnMaximumCharacterLengthAdvisor struct{}

// Check performs the ANTLR-based column maximum character length check using payload
func (a *ColumnMaximumCharacterLengthAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
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
	charLengthRule := NewColumnMaximumCharacterLengthRule(types.SQLReviewRuleLevel(level), string(rule.Type), maximum)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{charLengthRule})

	for _, stmtNode := range root {
		charLengthRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}