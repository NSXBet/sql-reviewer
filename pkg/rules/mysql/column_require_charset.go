package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ColumnRequireCharsetRule is the ANTLR-based implementation for checking require charset
type ColumnRequireCharsetRule struct {
	BaseAntlrRule
}

// NewColumnRequireCharsetRule creates a new ANTLR-based column require charset rule
func NewColumnRequireCharsetRule(level types.SQLReviewRuleLevel, title string) *ColumnRequireCharsetRule {
	return &ColumnRequireCharsetRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnRequireCharsetRule) Name() string {
	return "ColumnRequireCharsetRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnRequireCharsetRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnRequireCharsetRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnRequireCharsetRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil || ctx.TableElementList() == nil {
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
		columnDefinition := tableElement.ColumnDefinition()
		if columnDefinition.FieldDefinition() == nil || columnDefinition.FieldDefinition().DataType() == nil {
			continue
		}

		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		dataType := columnDefinition.FieldDefinition().DataType()
		if r.isCharsetDataType(dataType) {
			if dataType.CharsetWithOptBinary() == nil {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.NoCharset),
					Title:         r.title,
					Content:       fmt.Sprintf("Column %s does not have a character set specified", columnName),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + columnDefinition.GetStart().GetLine()),
				})
			}
		}
	}
}

func (r *ColumnRequireCharsetRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil ||
		ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}
	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		// Only check ADD COLUMN for now.
		if alterListItem.ADD_SYMBOL() == nil || alterListItem.COLUMN_SYMBOL() == nil || alterListItem.FieldDefinition() == nil {
			continue
		}

		columnName := mysqlparser.NormalizeMySQLIdentifier(alterListItem.Identifier())
		dataType := alterListItem.FieldDefinition().DataType()
		if r.isCharsetDataType(dataType) {
			if dataType.CharsetWithOptBinary() == nil {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.NoCharset),
					Title:         r.title,
					Content:       fmt.Sprintf("Column %s does not have a character set specified", columnName),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + alterListItem.GetStart().GetLine()),
				})
			}
		}
	}
}

func (*ColumnRequireCharsetRule) isCharsetDataType(dataType mysql.IDataTypeContext) bool {
	return dataType != nil && (dataType.CHAR_SYMBOL() != nil ||
		dataType.VARCHAR_SYMBOL() != nil ||
		dataType.VARYING_SYMBOL() != nil ||
		dataType.TINYTEXT_SYMBOL() != nil ||
		dataType.TEXT_SYMBOL() != nil ||
		dataType.MEDIUMTEXT_SYMBOL() != nil ||
		dataType.LONGTEXT_SYMBOL() != nil)
}

// ColumnRequireCharsetAdvisor is the advisor using ANTLR parser for column require charset checking
type ColumnRequireCharsetAdvisor struct{}

// Check performs the ANTLR-based column require charset check
func (a *ColumnRequireCharsetAdvisor) Check(
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

	// Create the rule
	columnRequireCharsetRule := NewColumnRequireCharsetRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{columnRequireCharsetRule})

	for _, stmtNode := range root {
		columnRequireCharsetRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
