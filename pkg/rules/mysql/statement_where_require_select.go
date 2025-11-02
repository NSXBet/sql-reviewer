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

// StatementWhereRequireSelectRule is the ANTLR-based implementation for checking WHERE clause requirement in SELECT statements
type StatementWhereRequireSelectRule struct {
	BaseAntlrRule
	text string
}

// NewStatementWhereRequireSelectRule creates a new ANTLR-based WHERE requirement rule for SELECT
func NewStatementWhereRequireSelectRule(level types.SQLReviewRuleLevel, title string) *StatementWhereRequireSelectRule {
	return &StatementWhereRequireSelectRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementWhereRequireSelectRule) Name() string {
	return "StatementWhereRequireSelectRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementWhereRequireSelectRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeQuerySpecification:
		r.checkQuerySpecification(ctx.(*mysql.QuerySpecificationContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementWhereRequireSelectRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementWhereRequireSelectRule) checkQuerySpecification(ctx *mysql.QuerySpecificationContext) {
	// Allow SELECT queries without a FROM clause to proceed, e.g. SELECT 1.
	if ctx.FromClause() == nil {
		return
	}
	if ctx.WhereClause() == nil || ctx.WhereClause().WHERE_SYMBOL() == nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.StatementNoWhere),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" requires WHERE clause", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// StatementWhereRequireSelectAdvisor is the advisor using ANTLR parser for WHERE requirement checking in SELECT
type StatementWhereRequireSelectAdvisor struct{}

// Check performs the ANTLR-based WHERE requirement check for SELECT statements
func (a *StatementWhereRequireSelectAdvisor) Check(
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
	whereRule := NewStatementWhereRequireSelectRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{whereRule})

	for _, stmtNode := range root {
		whereRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
