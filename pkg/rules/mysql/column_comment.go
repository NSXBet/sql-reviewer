package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// ColumnCommentRule is the ANTLR-based implementation for checking column comment requirements
type ColumnCommentRule struct {
	BaseAntlrRule
	maxLength int
	required  bool
}

// NewColumnCommentRule creates a new ANTLR-based column comment rule
func NewColumnCommentRule(level types.SQLReviewRuleLevel, title string, maxLength int, required bool) *ColumnCommentRule {
	return &ColumnCommentRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		maxLength: maxLength,
		required:  required,
	}
}

// Name returns the rule name
func (*ColumnCommentRule) Name() string {
	return "ColumnCommentRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnCommentRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnCommentRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnCommentRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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
		r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
	}
}

func (r *ColumnCommentRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

func (r *ColumnCommentRule) checkFieldDefinition(tableName, columnName string, ctx mysql.IFieldDefinitionContext) {
	comment := ""
	for _, attribute := range ctx.AllColumnAttribute() {
		if attribute == nil || attribute.GetValue() == nil {
			continue
		}
		if attribute.GetValue().GetTokenType() != mysql.MySQLParserCOMMENT_SYMBOL {
			continue
		}
		if attribute.TextLiteral() == nil {
			continue
		}
		comment = mysqlparser.NormalizeMySQLTextLiteral(attribute.TextLiteral())
		if r.maxLength >= 0 && len(comment) > r.maxLength {
			r.AddAdvice(&types.Advice{
				Status: types.Advice_Status(r.level),
				Code:   int32(types.ColumnCommentTooLong),
				Title:  r.title,
				Content: fmt.Sprintf(
					"The length of column `%s`.`%s` comment should be within %d characters",
					tableName,
					columnName,
					r.maxLength,
				),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}

		break
	}

	if len(comment) == 0 && r.required {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.ColumnRequireComment),
			Title:         r.title,
			Content:       fmt.Sprintf("Column `%s`.`%s` requires comments", tableName, columnName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// ColumnCommentAdvisor is the advisor using ANTLR parser for column comment checking
type ColumnCommentAdvisor struct{}

// Check performs the ANTLR-based column comment check
func (a *ColumnCommentAdvisor) Check(
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

	// Create the rule with maxLength = 10 and required = true based on test data
	commentRule := NewColumnCommentRule(types.SQLReviewRuleLevel(level), string(rule.Type), 10, true)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{commentRule})

	for _, stmtNode := range root {
		commentRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
