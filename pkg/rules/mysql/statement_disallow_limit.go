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

// StatementDisallowLimitRule is the ANTLR-based implementation for checking disallowed LIMIT clauses
type StatementDisallowLimitRule struct {
	BaseAntlrRule
	isInsertStmt bool
	text         string
	line         int //nolint:unused
}

// NewStatementDisallowLimitRule creates a new ANTLR-based statement disallow limit rule
func NewStatementDisallowLimitRule(level types.SQLReviewRuleLevel, title string) *StatementDisallowLimitRule {
	return &StatementDisallowLimitRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementDisallowLimitRule) Name() string {
	return "StatementDisallowLimitRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementDisallowLimitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeDeleteStatement:
		r.checkDeleteStatement(ctx.(*mysql.DeleteStatementContext))
	case NodeTypeUpdateStatement:
		r.checkUpdateStatement(ctx.(*mysql.UpdateStatementContext))
	case NodeTypeInsertStatement:
		r.isInsertStmt = true
	case NodeTypeQueryExpression:
		r.checkQueryExpression(ctx.(*mysql.QueryExpressionContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (r *StatementDisallowLimitRule) OnExit(_ antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeInsertStatement {
		r.isInsertStmt = false
	}
	return nil
}

func (r *StatementDisallowLimitRule) checkDeleteStatement(ctx *mysql.DeleteStatementContext) {
	if ctx.SimpleLimitClause() != nil && ctx.SimpleLimitClause().LIMIT_SYMBOL() != nil {
		r.handleLimitClause(types.DeleteUseLimit, ctx.GetStart().GetLine())
	}
}

func (r *StatementDisallowLimitRule) checkUpdateStatement(ctx *mysql.UpdateStatementContext) {
	if ctx.SimpleLimitClause() != nil && ctx.SimpleLimitClause().LIMIT_SYMBOL() != nil {
		r.handleLimitClause(types.UpdateUseLimit, ctx.GetStart().GetLine())
	}
}

func (r *StatementDisallowLimitRule) checkQueryExpression(ctx *mysql.QueryExpressionContext) {
	if !r.isInsertStmt {
		return
	}
	if ctx.LimitClause() != nil && ctx.LimitClause().LIMIT_SYMBOL() != nil {
		r.handleLimitClause(types.InsertUseLimit, ctx.GetStart().GetLine())
	}
}

func (r *StatementDisallowLimitRule) handleLimitClause(code int, lineNumber int) {
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(code),
		Title:         r.title,
		Content:       fmt.Sprintf("LIMIT clause is forbidden in INSERT, UPDATE and DELETE statement, but \"%s\" uses", r.text),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + lineNumber),
	})
}

// StatementDisallowLimitAdvisor is the advisor using ANTLR parser for statement disallow limit checking
type StatementDisallowLimitAdvisor struct{}

// Check performs the ANTLR-based statement disallow limit check
func (a *StatementDisallowLimitAdvisor) Check(
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
	limitRule := NewStatementDisallowLimitRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{limitRule})

	for _, stmtNode := range root {
		limitRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
