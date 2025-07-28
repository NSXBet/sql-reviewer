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

type StatementWhereMaximumLogicalOperatorCountAdvisor struct {
}

func (a *StatementWhereMaximumLogicalOperatorCountAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	var allAdvice []*types.Advice
	for _, stmt := range stmtList {
		// Create the rule for each statement
		logicalRule := NewStatementWhereMaximumLogicalOperatorCountRule(level, string(rule.Type), payload.Number)

		// Create the generic checker with the rule
		checker := NewGenericChecker([]Rule{logicalRule})

		logicalRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		logicalRule.resetForStatement()
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)

		// Check OR conditions after walking the tree
		logicalRule.checkOrConditions()

		allAdvice = append(allAdvice, checker.GetAdviceList()...)
	}

	return allAdvice, nil
}

// StatementWhereMaximumLogicalOperatorCountRule checks for maximum logical operators in WHERE.
type StatementWhereMaximumLogicalOperatorCountRule struct {
	BaseRule
	text              string
	maximum           int
	reported          bool
	depth             int
	inPredicateExprIn bool
	maxOrCount        int
	maxOrCountLine    int
}

// NewStatementWhereMaximumLogicalOperatorCountRule creates a new StatementWhereMaximumLogicalOperatorCountRule.
func NewStatementWhereMaximumLogicalOperatorCountRule(level types.Advice_Status, title string, maximum int) *StatementWhereMaximumLogicalOperatorCountRule {
	return &StatementWhereMaximumLogicalOperatorCountRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		maximum: maximum,
	}
}

// Name returns the rule name.
func (*StatementWhereMaximumLogicalOperatorCountRule) Name() string {
	return "StatementWhereMaximumLogicalOperatorCountRule"
}

// resetForStatement resets state for a new statement.
func (r *StatementWhereMaximumLogicalOperatorCountRule) resetForStatement() {
	r.reported = false
	r.depth = 0
	r.inPredicateExprIn = false
	r.maxOrCount = 0
	r.maxOrCountLine = 0
}

// OnEnter is called when entering a parse tree node.
func (r *StatementWhereMaximumLogicalOperatorCountRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypePredicateExprIn:
		r.inPredicateExprIn = true
	case NodeTypeExprList:
		r.checkExprList(ctx.(*mysql.ExprListContext))
	case NodeTypeExprOr:
		r.checkExprOr(ctx.(*mysql.ExprOrContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (r *StatementWhereMaximumLogicalOperatorCountRule) OnExit(_ antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypePredicateExprIn:
		r.inPredicateExprIn = false
	case NodeTypeExprOr:
		r.depth--
	}
	return nil
}

func (r *StatementWhereMaximumLogicalOperatorCountRule) checkExprList(ctx *mysql.ExprListContext) {
	if r.reported {
		return
	}
	if !r.inPredicateExprIn {
		return
	}

	count := len(ctx.AllExpr())
	if count > r.maximum {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.StatementWhereMaximumLogicalOperatorCount),
			Title:         r.title,
			Content:       fmt.Sprintf("Number of tokens (%d) in IN predicate operation exceeds limit (%d) in statement \"%s\".", count, r.maximum, r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *StatementWhereMaximumLogicalOperatorCountRule) checkExprOr(ctx *mysql.ExprOrContext) {
	r.depth++
	count := r.depth + 1
	if count > r.maxOrCount {
		r.maxOrCount = count
		r.maxOrCountLine = r.baseLine + ctx.GetStart().GetLine()
	}
}

func (r *StatementWhereMaximumLogicalOperatorCountRule) checkOrConditions() {
	if r.maxOrCount > r.maximum {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.StatementWhereMaximumLogicalOperatorCount),
			Title:         r.title,
			Content:       fmt.Sprintf("Number of tokens (%d) in the OR predicate operation exceeds limit (%d) in statement \"%s\".", r.maxOrCount, r.maximum, r.text),
			StartPosition: ConvertANTLRLineToPosition(r.maxOrCountLine),
		})
	}
}