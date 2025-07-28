package mysql

import (
	"context"
	"fmt"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"github.com/pkg/errors"
)

// NamingColumnAutoIncrementAdvisor is the ANTLR-based implementation for checking auto-increment naming convention
type NamingColumnAutoIncrementAdvisor struct {
	BaseAntlrRule
	format    *regexp.Regexp
	maxLength int
}

// NewNamingAutoIncrementColumnRule creates a new ANTLR-based auto-increment naming rule
func NewNamingAutoIncrementColumnRule(
	level types.SQLReviewRuleLevel,
	title string,
	format *regexp.Regexp,
	maxLength int,
) *NamingColumnAutoIncrementAdvisor {
	return &NamingColumnAutoIncrementAdvisor{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		format:    format,
		maxLength: maxLength,
	}
}

// Name returns the rule name
func (*NamingColumnAutoIncrementAdvisor) Name() string {
	return "NamingColumnAutoIncrementAdvisor"
}

// OnEnter is called when entering a parse tree node
func (r *NamingColumnAutoIncrementAdvisor) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*NamingColumnAutoIncrementAdvisor) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *NamingColumnAutoIncrementAdvisor) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil || ctx.TableElementList() == nil {
		return
	}

	tableName := NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		if tableElement.ColumnDefinition() == nil {
			continue
		}
		if tableElement.ColumnDefinition().ColumnName() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil {
			continue
		}

		columnName := NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
	}
}

func (r *NamingColumnAutoIncrementAdvisor) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	tableName := NormalizeMySQLTableRef(ctx.TableRef())
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
				columnName := NormalizeMySQLIdentifier(item.Identifier())
				r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil ||
						tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					columnName := NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.checkFieldDefinition(tableName, columnName, tableElement.ColumnDefinition().FieldDefinition())
				}
			}
		// modify column
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			columnName := NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		// change column
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			columnName := NormalizeMySQLIdentifier(item.Identifier())
			r.checkFieldDefinition(tableName, columnName, item.FieldDefinition())
		}
	}
}

func (r *NamingColumnAutoIncrementAdvisor) checkFieldDefinition(tableName, columnName string, ctx mysql.IFieldDefinitionContext) {
	if !r.isAutoIncrement(ctx) {
		return
	}

	if !r.format.MatchString(columnName) {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.NamingAutoIncrementColumnConventionMismatch),
			Title:  r.title,
			Content: fmt.Sprintf(
				"`%s`.`%s` mismatches auto_increment column naming convention, naming format should be %q",
				tableName,
				columnName,
				r.format,
			),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
	if r.maxLength > 0 && len(columnName) > r.maxLength {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.NamingAutoIncrementColumnConventionMismatch),
			Title:  r.title,
			Content: fmt.Sprintf(
				"`%s`.`%s` mismatches auto_increment column naming convention, its length should be within %d characters",
				tableName,
				columnName,
				r.maxLength,
			),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (*NamingColumnAutoIncrementAdvisor) isAutoIncrement(ctx mysql.IFieldDefinitionContext) bool {
	for _, attr := range ctx.AllColumnAttribute() {
		if attr.AUTO_INCREMENT_SYMBOL() != nil {
			return true
		}
	}
	return false
}

// NamingAutoIncrementColumnAdvisor is the advisor using ANTLR parser for auto-increment naming convention checking
type NamingAutoIncrementColumnAdvisor struct{}

// Check performs the ANTLR-based auto-increment naming convention check using payload
func (a *NamingAutoIncrementColumnAdvisor) Check(
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

	// Parse the regex pattern from rule payload
	payload, err := advisor.UnmarshalStringTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	pattern := payload.String
	maxLength := 64 // Default max length

	format, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid naming pattern %q", pattern)
	}

	// Create the rule with naming pattern
	namingRule := NewNamingAutoIncrementColumnRule(types.SQLReviewRuleLevel(level), string(rule.Type), format, maxLength)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{namingRule})

	for _, stmtNode := range root {
		namingRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
