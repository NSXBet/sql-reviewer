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

// StatementQueryMinumumPlanLevelRule is the ANTLR-based implementation for checking query minimum plan level
type StatementQueryMinumumPlanLevelRule struct {
	BaseAntlrRule
	explainType string
}

// NewStatementQueryMinumumPlanLevelRule creates a new ANTLR-based statement query minimum plan level rule
func NewStatementQueryMinumumPlanLevelRule(
	level types.SQLReviewRuleLevel,
	title string,
	explainType string,
) *StatementQueryMinumumPlanLevelRule {
	return &StatementQueryMinumumPlanLevelRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		explainType: explainType,
	}
}

// Name returns the rule name
func (*StatementQueryMinumumPlanLevelRule) Name() string {
	return "StatementQueryMinumumPlanLevelRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementQueryMinumumPlanLevelRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeSelectStatement:
		r.checkSelectStatement(ctx.(*mysql.SelectStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementQueryMinumumPlanLevelRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementQueryMinumumPlanLevelRule) checkSelectStatement(ctx *mysql.SelectStatementContext) {
	// For the CLI implementation, we don't have a database connection to run EXPLAIN queries
	// This is a simplified version that reports a warning for any SELECT statement
	// advising users to check query plans manually
	r.AddAdvice(&types.Advice{
		Status: types.Advice_Status(r.level),
		Code:   int32(types.StatementUnwantedQueryPlanLevel),
		Title:  r.title,
		Content: fmt.Sprintf(
			"Query plan level check required. Please verify that this query meets the minimum plan level requirement (%s) using EXPLAIN.",
			r.explainType,
		),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
	})
}

// StatementQueryMinumumPlanLevelAdvisor is the advisor using ANTLR parser for statement query minimum plan level checking
type StatementQueryMinumumPlanLevelAdvisor struct{}

// Check performs the ANTLR-based statement query minimum plan level check
func (a *StatementQueryMinumumPlanLevelAdvisor) Check(
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

	payload, err := advisor.UnmarshalStringTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	// Create the rule
	statementQueryMinumumPlanLevelRule := NewStatementQueryMinumumPlanLevelRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
		payload.String,
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{statementQueryMinumumPlanLevelRule})

	for _, stmtNode := range root {
		statementQueryMinumumPlanLevelRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
