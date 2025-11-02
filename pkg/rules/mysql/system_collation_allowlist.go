package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type SystemCollationAllowlistAdvisor struct{}

func (a *SystemCollationAllowlistAdvisor) Check(
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

	// Create the rule
	collationRule := NewCollationAllowlistRule(level, string(rule.Type), payload.List)

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{collationRule})

	for _, stmt := range stmtList {
		collationRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		// Set text will be handled in the Query node
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

// CollationAllowlistRule checks for collation allowlist.
type CollationAllowlistRule struct {
	BaseRule
	allowList map[string]bool
	text      string
}

// NewCollationAllowlistRule creates a new CollationAllowlistRule.
func NewCollationAllowlistRule(level types.Advice_Status, title string, allowList []string) *CollationAllowlistRule {
	rule := &CollationAllowlistRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		allowList: make(map[string]bool),
	}
	for _, collation := range allowList {
		rule.allowList[strings.ToLower(collation)] = true
	}
	return rule
}

// Name returns the rule name.
func (*CollationAllowlistRule) Name() string {
	return "CollationAllowlistRule"
}

// SetText sets the query text for error reporting.
func (r *CollationAllowlistRule) SetText(text string) {
	r.text = text
}

// OnEnter is called when entering a parse tree node.
func (r *CollationAllowlistRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		r.checkQuery(ctx.(*mysql.QueryContext))
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
func (*CollationAllowlistRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *CollationAllowlistRule) checkQuery(ctx *mysql.QueryContext) {
	r.text = ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
}

func (r *CollationAllowlistRule) checkCreateDatabase(ctx *mysql.CreateDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	for _, option := range ctx.AllCreateDatabaseOption() {
		if option != nil && option.DefaultCollation() != nil && option.DefaultCollation().CollationName() != nil {
			collation := mysqlparser.NormalizeMySQLCollationName(option.DefaultCollation().CollationName())
			r.checkCollation(collation, ctx.GetStart().GetLine())
		}
	}
}

func (r *CollationAllowlistRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.CreateTableOptions() != nil {
		for _, option := range ctx.CreateTableOptions().AllCreateTableOption() {
			if option != nil && option.DefaultCollation() != nil && option.DefaultCollation().CollationName() != nil {
				collation := mysqlparser.NormalizeMySQLCollationName(option.DefaultCollation().CollationName())
				r.checkCollation(collation, option.GetStart().GetLine())
			}
		}
	}

	if ctx.TableElementList() != nil {
		for _, tableElement := range ctx.TableElementList().AllTableElement() {
			if tableElement == nil || tableElement.ColumnDefinition() == nil {
				continue
			}
			columnDef := tableElement.ColumnDefinition()
			if columnDef.FieldDefinition() == nil {
				continue
			}
			if columnDef.FieldDefinition().AllColumnAttribute() == nil {
				continue
			}
			for _, attr := range columnDef.FieldDefinition().AllColumnAttribute() {
				if attr != nil && attr.Collate() != nil && attr.Collate().CollationName() != nil {
					collation := mysqlparser.NormalizeMySQLCollationName(attr.Collate().CollationName())
					r.checkCollation(collation, tableElement.GetStart().GetLine())
				}
			}
		}
	}
}

func (r *CollationAllowlistRule) checkAlterDatabase(ctx *mysql.AlterDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	for _, option := range ctx.AllAlterDatabaseOption() {
		if option == nil || option.CreateDatabaseOption() == nil || option.CreateDatabaseOption().DefaultCollation() == nil ||
			option.CreateDatabaseOption().DefaultCollation().CollationName() == nil {
			continue
		}
		collation := mysqlparser.NormalizeMySQLCollationName(option.CreateDatabaseOption().DefaultCollation().CollationName())
		r.checkCollation(collation, ctx.GetStart().GetLine())
	}
}

func (r *CollationAllowlistRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil || item.FieldDefinition() == nil {
			continue
		}
		for _, attr := range item.FieldDefinition().AllColumnAttribute() {
			if attr == nil || attr.Collate() == nil || attr.Collate().CollationName() == nil {
				continue
			}
			collation := mysqlparser.NormalizeMySQLCollationName(attr.Collate().CollationName())
			r.checkCollation(collation, item.GetStart().GetLine())
		}
	}
	// alter table option
	for _, option := range ctx.AlterTableActions().AlterCommandList().AlterList().AllCreateTableOptionsSpaceSeparated() {
		if option == nil {
			continue
		}
		for _, tableOption := range option.AllCreateTableOption() {
			if tableOption == nil {
				continue
			}
			if tableOption.DefaultCollation() == nil || tableOption.DefaultCollation().CollationName() == nil {
				continue
			}
			collation := mysqlparser.NormalizeMySQLCollationName(tableOption.DefaultCollation().CollationName())
			r.checkCollation(collation, tableOption.GetStart().GetLine())
		}
	}
}

func (r *CollationAllowlistRule) checkCollation(collation string, lineNumber int) {
	collation = strings.ToLower(collation)
	if _, exists := r.allowList[collation]; collation != "" && !exists {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.DisabledCollation),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" used disabled collation '%s'", r.text, collation),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + lineNumber),
		})
	}
}
