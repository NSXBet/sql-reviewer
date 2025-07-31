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

// ColumnDisallowSetCharsetRule is the ANTLR-based implementation for checking disallow set column charset
type ColumnDisallowSetCharsetRule struct {
	BaseAntlrRule
	text string
}

// NewColumnDisallowSetCharsetRule creates a new ANTLR-based column disallow set charset rule
func NewColumnDisallowSetCharsetRule(level types.SQLReviewRuleLevel, title string) *ColumnDisallowSetCharsetRule {
	return &ColumnDisallowSetCharsetRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*ColumnDisallowSetCharsetRule) Name() string {
	return "ColumnDisallowSetCharsetRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnDisallowSetCharsetRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		r.checkQuery(ctx.(*mysql.QueryContext))
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnDisallowSetCharsetRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnDisallowSetCharsetRule) checkQuery(ctx *mysql.QueryContext) {
	r.text = ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
}

func (r *ColumnDisallowSetCharsetRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableElementList() == nil || ctx.TableName() == nil {
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
		charset := r.getCharset(tableElement.ColumnDefinition().FieldDefinition().DataType())
		if !r.checkCharset(charset) {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.SetColumnCharset),
				Title:         r.title,
				Content:       fmt.Sprintf("Disallow set column charset but \"%s\" does", r.text),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

func (r *ColumnDisallowSetCharsetRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

		var charsetList []string
		switch {
		// add column.
		case item.ADD_SYMBOL() != nil:
			switch {
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				if item.FieldDefinition().DataType() == nil {
					continue
				}
				charsetName := r.getCharset(item.FieldDefinition().DataType())
				charsetList = append(charsetList, charsetName)
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil {
						continue
					}
					if tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					if tableElement.ColumnDefinition().FieldDefinition().DataType() == nil {
						continue
					}
					charsetName := r.getCharset(tableElement.ColumnDefinition().FieldDefinition().DataType())
					charsetList = append(charsetList, charsetName)
				}
			}
		// change column.
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			charsetName := r.getCharset(item.FieldDefinition().DataType())
			charsetList = append(charsetList, charsetName)
		// modify column.
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			if item.FieldDefinition().DataType() == nil {
				continue
			}
			charsetName := r.getCharset(item.FieldDefinition().DataType())
			charsetList = append(charsetList, charsetName)
		default:
			continue
		}

		for _, charsetName := range charsetList {
			if !r.checkCharset(charsetName) {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.SetColumnCharset),
					Title:         r.title,
					Content:       fmt.Sprintf("Disallow set column charset but \"%s\" does", r.text),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
				})
			}
		}
	}
}

func (*ColumnDisallowSetCharsetRule) getCharset(ctx mysql.IDataTypeContext) string {
	if ctx.CharsetWithOptBinary() == nil {
		return ""
	}
	charset := mysqlparser.NormalizeMySQLCharsetName(ctx.CharsetWithOptBinary().CharsetName())
	return charset
}

func (*ColumnDisallowSetCharsetRule) checkCharset(charset string) bool {
	switch charset {
	// empty charset or binary for JSON.
	case "", "binary":
		return true
	default:
		return false
	}
}

// ColumnDisallowSetCharsetAdvisor is the advisor using ANTLR parser for column disallow set charset checking
type ColumnDisallowSetCharsetAdvisor struct{}

// Check performs the ANTLR-based column disallow set charset check
func (a *ColumnDisallowSetCharsetAdvisor) Check(
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

	// Create the rule (doesn't need catalog)
	charsetRule := NewColumnDisallowSetCharsetRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{charsetRule})

	for _, stmtNode := range root {
		charsetRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
