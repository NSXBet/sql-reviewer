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

// TableDisallowSetCharsetRule is the ANTLR-based implementation for checking disallowed table charset setting
type TableDisallowSetCharsetRule struct {
	BaseAntlrRule
	text string
}

// NewTableDisallowSetCharsetRule creates a new ANTLR-based table disallow set charset rule
func NewTableDisallowSetCharsetRule(level types.SQLReviewRuleLevel, title string) *TableDisallowSetCharsetRule {
	return &TableDisallowSetCharsetRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*TableDisallowSetCharsetRule) Name() string {
	return "TableDisallowSetCharsetRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableDisallowSetCharsetRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableDisallowSetCharsetRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableDisallowSetCharsetRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.CreateTableOptions() != nil {
		for _, option := range ctx.CreateTableOptions().AllCreateTableOption() {
			if option.DefaultCharset() != nil {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.DisallowSetCharset),
					Title:         r.title,
					Content:       fmt.Sprintf("Set charset on tables is disallowed, but \"%s\" uses", r.text),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
				})
			}
		}
	}
}

func (r *TableDisallowSetCharsetRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}

	alterList := ctx.AlterTableActions().AlterCommandList().AlterList()
	if alterList == nil {
		return
	}
	for _, alterListItem := range alterList.AllAlterListItem() {
		if alterListItem == nil {
			continue
		}

		if alterListItem.Charset() != nil {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.DisallowSetCharset),
				Title:         r.title,
				Content:       fmt.Sprintf("Set charset on tables is disallowed, but \"%s\" uses", r.text),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// TableDisallowSetCharsetAdvisor is the advisor using ANTLR parser for table disallow set charset checking
type TableDisallowSetCharsetAdvisor struct{}

// Check performs the ANTLR-based table disallow set charset check
func (a *TableDisallowSetCharsetAdvisor) Check(
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
	charsetRule := NewTableDisallowSetCharsetRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{charsetRule})

	for _, stmtNode := range root {
		charsetRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
