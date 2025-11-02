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

// StatementWhereRequireUpdateDeleteRule is the ANTLR-based implementation for checking WHERE clause requirement in UPDATE/DELETE statements
type StatementWhereRequireUpdateDeleteRule struct {
	BaseAntlrRule
	text string
}

// NewStatementWhereRequireUpdateDeleteRule creates a new ANTLR-based WHERE requirement rule for UPDATE/DELETE
func NewStatementWhereRequireUpdateDeleteRule(
	level types.SQLReviewRuleLevel,
	title string,
) *StatementWhereRequireUpdateDeleteRule {
	return &StatementWhereRequireUpdateDeleteRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementWhereRequireUpdateDeleteRule) Name() string {
	return "StatementWhereRequireUpdateDeleteRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementWhereRequireUpdateDeleteRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeDeleteStatement:
		r.checkDeleteStatement(ctx.(*mysql.DeleteStatementContext))
	case NodeTypeUpdateStatement:
		r.checkUpdateStatement(ctx.(*mysql.UpdateStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementWhereRequireUpdateDeleteRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementWhereRequireUpdateDeleteRule) checkDeleteStatement(ctx *mysql.DeleteStatementContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.WhereClause() == nil || ctx.WhereClause().WHERE_SYMBOL() == nil {
		r.handleWhereClause(ctx.GetStart().GetLine())
	}
}

func (r *StatementWhereRequireUpdateDeleteRule) checkUpdateStatement(ctx *mysql.UpdateStatementContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.WhereClause() == nil || ctx.WhereClause().WHERE_SYMBOL() == nil {
		r.handleWhereClause(ctx.GetStart().GetLine())
	}
}

func (r *StatementWhereRequireUpdateDeleteRule) handleWhereClause(lineNumber int) {
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.StatementNoWhere),
		Title:         r.title,
		Content:       fmt.Sprintf("\"%s\" requires WHERE clause", r.text),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + lineNumber),
	})
}

// StatementWhereRequireUpdateDeleteAdvisor is the advisor using ANTLR parser for WHERE requirement checking in UPDATE/DELETE
type StatementWhereRequireUpdateDeleteAdvisor struct{}

// Check performs the ANTLR-based WHERE requirement check for UPDATE/DELETE statements
func (a *StatementWhereRequireUpdateDeleteAdvisor) Check(
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
	whereRule := NewStatementWhereRequireUpdateDeleteRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{whereRule})

	for _, stmtNode := range root {
		whereRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
