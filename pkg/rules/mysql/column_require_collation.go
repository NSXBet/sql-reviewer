package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ColumnRequireCollationRule is the ANTLR-based implementation for checking require collation
type ColumnRequireCollationRule struct {
	BaseAntlrRule
}

// NewColumnRequireCollationRule creates a new ANTLR-based column require collation rule
func NewColumnRequireCollationRule(level types.SQLReviewRuleLevel, title string) *ColumnRequireCollationRule {
	return &ColumnRequireCollationRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnRequireCollationRule) Name() string {
	return "ColumnRequireCollationRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnRequireCollationRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnRequireCollationRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnRequireCollationRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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
			// Check if any column attribute has collation
			hasCollation := false
			if columnDefinition.FieldDefinition().AllColumnAttribute() != nil {
				for _, attr := range columnDefinition.FieldDefinition().AllColumnAttribute() {
					if attr != nil && attr.Collate() != nil {
						hasCollation = true
						break
					}
				}
			}
			if !hasCollation {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.NoCollation),
					Title:         r.title,
					Content:       fmt.Sprintf("Column %s does not have a collation specified", columnName),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + columnDefinition.GetStart().GetLine()),
				})
			}
		}
	}
}

func (r *ColumnRequireCollationRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
			// Check if any column attribute has collation
			hasCollation := false
			if alterListItem.FieldDefinition().AllColumnAttribute() != nil {
				for _, attr := range alterListItem.FieldDefinition().AllColumnAttribute() {
					if attr != nil && attr.Collate() != nil {
						hasCollation = true
						break
					}
				}
			}
			if !hasCollation {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.NoCollation),
					Title:         r.title,
					Content:       fmt.Sprintf("Column %s does not have a collation specified", columnName),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + alterListItem.GetStart().GetLine()),
				})
			}
		}
	}
}

func (*ColumnRequireCollationRule) isCharsetDataType(dataType mysql.IDataTypeContext) bool {
	return dataType != nil && (dataType.CHAR_SYMBOL() != nil ||
		dataType.VARCHAR_SYMBOL() != nil ||
		dataType.VARYING_SYMBOL() != nil ||
		dataType.TINYTEXT_SYMBOL() != nil ||
		dataType.TEXT_SYMBOL() != nil ||
		dataType.MEDIUMTEXT_SYMBOL() != nil ||
		dataType.LONGTEXT_SYMBOL() != nil)
}

// ColumnRequireCollationAdvisor is the advisor using ANTLR parser for column require collation checking
type ColumnRequireCollationAdvisor struct{}

// Check performs the ANTLR-based column require collation check
func (a *ColumnRequireCollationAdvisor) Check(
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
	columnRequireCollationRule := NewColumnRequireCollationRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{columnRequireCollationRule})

	for _, stmtNode := range root {
		columnRequireCollationRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
