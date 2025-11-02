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

// StatementSelectNoSelectAllRule is the ANTLR-based implementation for checking SELECT * usage
type StatementSelectNoSelectAllRule struct {
	BaseAntlrRule
	text string
}

// NewStatementSelectNoSelectAllRule creates a new ANTLR-based SELECT * rule
func NewStatementSelectNoSelectAllRule(level types.SQLReviewRuleLevel, title string) *StatementSelectNoSelectAllRule {
	return &StatementSelectNoSelectAllRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementSelectNoSelectAllRule) Name() string {
	return "StatementSelectNoSelectAllRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementSelectNoSelectAllRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeSelectItemList:
		r.checkSelectItemList(ctx.(*mysql.SelectItemListContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementSelectNoSelectAllRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementSelectNoSelectAllRule) checkSelectItemList(ctx *mysql.SelectItemListContext) {
	if ctx.MULT_OPERATOR() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.StatementSelectAll),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" uses SELECT all", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// StatementSelectNoSelectAllAdvisor is the advisor using ANTLR parser for SELECT * checking
type StatementSelectNoSelectAllAdvisor struct{}

// Check performs the ANTLR-based SELECT * check
func (a *StatementSelectNoSelectAllAdvisor) Check(
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
	selectRule := NewStatementSelectNoSelectAllRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{selectRule})

	for _, stmtNode := range root {
		selectRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
