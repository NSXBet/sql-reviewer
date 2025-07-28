package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type SystemCharsetAllowlistAdvisor struct{}

func (a *SystemCharsetAllowlistAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	allowList := make(map[string]bool)
	for _, charset := range payload.List {
		allowList[strings.ToLower(charset)] = true
	}

	// Create the rule
	charsetRule := NewCharsetAllowlistRule(level, string(rule.Type), allowList)

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{charsetRule})

	for _, stmt := range stmtList {
		charsetRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

// CharsetAllowlistRule checks for charset allowlist.
type CharsetAllowlistRule struct {
	BaseRule
	text      string
	allowList map[string]bool
}

// NewCharsetAllowlistRule creates a new CharsetAllowlistRule.
func NewCharsetAllowlistRule(level types.Advice_Status, title string, allowList map[string]bool) *CharsetAllowlistRule {
	return &CharsetAllowlistRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		allowList: allowList,
	}
}

// Name returns the rule name.
func (*CharsetAllowlistRule) Name() string {
	return "CharsetAllowlistRule"
}

// OnEnter is called when entering a parse tree node.
func (r *CharsetAllowlistRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeCreateDatabase:
		r.checkCreateDatabase(ctx.(*mysql.CreateDatabaseContext))
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterDatabase:
		r.checkAlterDatabase(ctx.(*mysql.AlterDatabaseContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*CharsetAllowlistRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *CharsetAllowlistRule) checkCreateDatabase(ctx *mysql.CreateDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	for _, option := range ctx.AllCreateDatabaseOption() {
		if option.DefaultCharset() != nil {
			charset := mysqlparser.NormalizeMySQLCharsetName(option.DefaultCharset().CharsetName())
			charset = strings.ToLower(charset)
			r.checkCharset(charset, ctx.GetStart().GetLine())
			break
		}
	}
}

func (r *CharsetAllowlistRule) checkCharset(charset string, lineNumber int) {
	if _, exists := r.allowList[charset]; charset != "" && !exists {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.DisabledCharset),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" used disabled charset '%s'", r.text, charset),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + lineNumber),
		})
	}
}

func (r *CharsetAllowlistRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.CreateTableOptions() != nil {
		for _, option := range ctx.CreateTableOptions().AllCreateTableOption() {
			if option.DefaultCharset() != nil {
				charset := mysqlparser.NormalizeMySQLCharsetName(option.DefaultCharset().CharsetName())
				charset = strings.ToLower(charset)
				r.checkCharset(charset, ctx.GetStart().GetLine())
				break
			}
		}
	}

	if ctx.TableElementList() != nil {
		for _, tableElement := range ctx.TableElementList().AllTableElement() {
			if tableElement.ColumnDefinition() != nil {
				if tableElement.ColumnDefinition() == nil {
					continue
				}
				columnDef := tableElement.ColumnDefinition()
				if columnDef.FieldDefinition() == nil || columnDef.FieldDefinition().DataType() == nil {
					continue
				}
				if columnDef.FieldDefinition().DataType().CharsetWithOptBinary() == nil {
					continue
				}
				charsetName := columnDef.FieldDefinition().DataType().CharsetWithOptBinary().CharsetName()
				charset := mysqlparser.NormalizeMySQLCharsetName(charsetName)
				charset = strings.ToLower(charset)
				r.checkCharset(charset, ctx.GetStart().GetLine())
			}
		}
	}
}

func (r *CharsetAllowlistRule) checkAlterDatabase(ctx *mysql.AlterDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	for _, option := range ctx.AllAlterDatabaseOption() {
		if option.CreateDatabaseOption() == nil || option.CreateDatabaseOption().DefaultCharset() == nil {
			continue
		}
		charset := mysqlparser.NormalizeMySQLCharsetName(option.CreateDatabaseOption().DefaultCharset().CharsetName())
		charset = strings.ToLower(charset)
		r.checkCharset(charset, ctx.GetStart().GetLine())
	}
}

func (r *CharsetAllowlistRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	// alter table add column, change column, modify column.
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil || item.FieldDefinition() == nil {
			continue
		}
		if item.FieldDefinition().DataType() == nil {
			continue
		}
		if item.FieldDefinition().DataType().CharsetWithOptBinary() == nil {
			continue
		}
		charset := mysqlparser.NormalizeMySQLCharsetName(item.FieldDefinition().DataType().CharsetWithOptBinary().CharsetName())
		charset = strings.ToLower(charset)
		r.checkCharset(charset, ctx.GetStart().GetLine())
	}
	// alter table option
	for _, option := range ctx.AlterTableActions().AlterCommandList().AlterList().AllCreateTableOptionsSpaceSeparated() {
		if option == nil {
			continue
		}
		for _, tableOption := range option.AllCreateTableOption() {
			if tableOption == nil || tableOption.DefaultCharset() == nil {
				continue
			}
			charset := mysqlparser.NormalizeMySQLCharsetName(tableOption.DefaultCharset().CharsetName())
			charset = strings.ToLower(charset)
			r.checkCharset(charset, ctx.GetStart().GetLine())
		}
	}
}
