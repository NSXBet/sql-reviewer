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

type StatementWhereDisallowFunctionsAndCalculationsAdvisor struct{}

func (a *StatementWhereDisallowFunctionsAndCalculationsAdvisor) Check(
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

	// Create the rule
	whereRule := NewStatementWhereDisallowFunctionsAndCalculationsRule(level, string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{whereRule})

	for _, stmt := range stmtList {
		whereRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

// StatementWhereDisallowFunctionsAndCalculationsRule checks for functions in WHERE clause.
type StatementWhereDisallowFunctionsAndCalculationsRule struct {
	BaseRule
	text          string
	isSelect      bool
	inWhereClause bool
}

// NewStatementWhereDisallowFunctionsAndCalculationsRule creates a new StatementWhereDisallowFunctionsAndCalculationsRule.
func NewStatementWhereDisallowFunctionsAndCalculationsRule(
	level types.Advice_Status,
	title string,
) *StatementWhereDisallowFunctionsAndCalculationsRule {
	return &StatementWhereDisallowFunctionsAndCalculationsRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name.
func (*StatementWhereDisallowFunctionsAndCalculationsRule) Name() string {
	return "StatementWhereDisallowFunctionsAndCalculationsRule"
}

// OnEnter is called when entering a parse tree node.
func (r *StatementWhereDisallowFunctionsAndCalculationsRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeSelectStatement:
		r.isSelect = true
	case NodeTypeWhereClause:
		r.inWhereClause = true
	case NodeTypeFunctionCall:
		r.checkFunctionCall(ctx.(*mysql.FunctionCallContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (r *StatementWhereDisallowFunctionsAndCalculationsRule) OnExit(_ antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeSelectStatement:
		r.isSelect = false
	case NodeTypeWhereClause:
		r.inWhereClause = false
	}
	return nil
}

func (r *StatementWhereDisallowFunctionsAndCalculationsRule) checkFunctionCall(ctx *mysql.FunctionCallContext) {
	if !r.isSelect || !r.inWhereClause {
		return
	}

	pi := ctx.PureIdentifier()
	if pi != nil {
		r.adviceList = append(r.adviceList, &types.Advice{
			Status:        r.level,
			Code:          int32(types.DisabledFunction),
			Title:         r.title,
			Content:       fmt.Sprintf("Function is disallowed in where clause, but \"%s\" uses", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}
