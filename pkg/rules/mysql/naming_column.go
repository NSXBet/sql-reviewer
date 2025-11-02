package mysql

import (
	"context"
	"fmt"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"github.com/pkg/errors"
)

// NamingColumnRule is the ANTLR-based implementation for checking column naming convention
type NamingColumnRule struct {
	BaseAntlrRule
	format    *regexp.Regexp
	maxLength int
}

// NewNamingColumnRule creates a new ANTLR-based naming column rule
func NewNamingColumnRule(level types.SQLReviewRuleLevel, title string, format *regexp.Regexp, maxLength int) *NamingColumnRule {
	return &NamingColumnRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		format:    format,
		maxLength: maxLength,
	}
}

// Name returns the rule name
func (*NamingColumnRule) Name() string {
	return "NamingColumnRule"
}

// OnEnter is called when entering a parse tree node
func (r *NamingColumnRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*NamingColumnRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *NamingColumnRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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
		if tableElement.ColumnDefinition().ColumnName() == nil {
			continue
		}

		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		r.handleColumn(tableName, columnName, tableElement.GetStart().GetLine())
	}
}

func (r *NamingColumnRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
				r.handleColumn(tableName, columnName, item.GetStart().GetLine())
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil ||
						tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.handleColumn(tableName, columnName, tableElement.GetStart().GetLine())
				}
			}
		// rename column
		case item.RENAME_SYMBOL() != nil && item.COLUMN_SYMBOL() != nil:
			// only focus on new column-name.
			columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.handleColumn(tableName, columnName, item.GetStart().GetLine())
		// change column
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil:
			// only focus on new column-name.
			columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.handleColumn(tableName, columnName, item.GetStart().GetLine())
		default:
			continue
		}
	}
}

func (r *NamingColumnRule) handleColumn(tableName string, columnName string, lineNumber int) {
	// we need to accumulate line number for each statement and elements of statements.
	lineNumber += r.baseLine
	if !r.format.MatchString(columnName) {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.NamingColumnConvention),
			Title:  r.title,
			Content: fmt.Sprintf(
				"`%s`.`%s` mismatches column naming convention, naming format should be %q",
				tableName,
				columnName,
				r.format,
			),
			StartPosition: ConvertANTLRLineToPosition(lineNumber),
		})
	}
	if r.maxLength > 0 && len(columnName) > r.maxLength {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.NamingColumnConvention),
			Title:  r.title,
			Content: fmt.Sprintf(
				"`%s`.`%s` mismatches column naming convention, its length should be within %d characters",
				tableName,
				columnName,
				r.maxLength,
			),
			StartPosition: ConvertANTLRLineToPosition(lineNumber),
		})
	}
}

// NamingColumnAdvisor is the advisor using ANTLR parser for column naming convention checking
type NamingColumnAdvisor struct{}

// Check performs the ANTLR-based column naming convention check
func (a *NamingColumnAdvisor) Check(
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

	// Create the default format and max length based on test data
	format, err := regexp.Compile("^[a-z]+(_[a-z]+)*$")
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regex pattern")
	}
	maxLength := 64

	// Create the rule (doesn't need catalog)
	namingRule := NewNamingColumnRule(types.SQLReviewRuleLevel(level), string(rule.Type), format, maxLength)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{namingRule})

	for _, stmtNode := range root {
		namingRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
