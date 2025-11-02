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

// StatementDisallowOrderByRule is the ANTLR-based implementation for checking disallowed ORDER BY clauses
type StatementDisallowOrderByRule struct {
	BaseAntlrRule
	text string
}

// NewStatementDisallowOrderByRule creates a new ANTLR-based statement disallow order by rule
func NewStatementDisallowOrderByRule(level types.SQLReviewRuleLevel, title string) *StatementDisallowOrderByRule {
	return &StatementDisallowOrderByRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementDisallowOrderByRule) Name() string {
	return "StatementDisallowOrderByRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementDisallowOrderByRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
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
func (*StatementDisallowOrderByRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementDisallowOrderByRule) checkDeleteStatement(ctx *mysql.DeleteStatementContext) {
	if ctx.OrderClause() != nil && ctx.OrderClause().ORDER_SYMBOL() != nil {
		r.handleOrderByClause(types.DeleteUseOrderBy, ctx.GetStart().GetLine())
	}
}

func (r *StatementDisallowOrderByRule) checkUpdateStatement(ctx *mysql.UpdateStatementContext) {
	if ctx.OrderClause() != nil && ctx.OrderClause().ORDER_SYMBOL() != nil {
		r.handleOrderByClause(types.UpdateUseOrderBy, ctx.GetStart().GetLine())
	}
}

func (r *StatementDisallowOrderByRule) handleOrderByClause(code int, lineNumber int) {
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(code),
		Title:         r.title,
		Content:       fmt.Sprintf("ORDER BY clause is forbidden in DELETE and UPDATE statements, but \"%s\" uses", r.text),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + lineNumber),
	})
}

// StatementDisallowOrderByAdvisor is the advisor using ANTLR parser for statement disallow order by checking
type StatementDisallowOrderByAdvisor struct{}

// Check performs the ANTLR-based statement disallow order by check
func (a *StatementDisallowOrderByAdvisor) Check(
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
	orderByRule := NewStatementDisallowOrderByRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{orderByRule})

	for _, stmtNode := range root {
		orderByRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
