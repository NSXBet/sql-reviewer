package mysql

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// StatementDisallowUsingTemporaryRule is the ANTLR-based implementation for checking using temporary
type StatementDisallowUsingTemporaryRule struct {
	BaseAntlrRule
}

// NewStatementDisallowUsingTemporaryRule creates a new ANTLR-based statement disallow using temporary rule
func NewStatementDisallowUsingTemporaryRule(level types.SQLReviewRuleLevel, title string) *StatementDisallowUsingTemporaryRule {
	return &StatementDisallowUsingTemporaryRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementDisallowUsingTemporaryRule) Name() string {
	return "StatementDisallowUsingTemporaryRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementDisallowUsingTemporaryRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeSelectStatement:
		r.checkSelectStatement(ctx.(*mysql.SelectStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementDisallowUsingTemporaryRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementDisallowUsingTemporaryRule) checkSelectStatement(ctx *mysql.SelectStatementContext) {
	// For the CLI implementation, we don't have a database connection to run EXPLAIN queries
	// This is a simplified version that reports a warning for any SELECT statement
	// advising users to check for temporary table usage manually
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.StatementHasUsingTemporary),
		Title:         r.title,
		Content:       "Using temporary table check required. Please verify that this query does not use temporary tables by running EXPLAIN and checking the 'Extra' column.",
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
	})
}

// StatementDisallowUsingTemporaryAdvisor is the advisor using ANTLR parser for statement disallow using temporary checking
type StatementDisallowUsingTemporaryAdvisor struct{}

// Check performs the ANTLR-based statement disallow using temporary check
func (a *StatementDisallowUsingTemporaryAdvisor) Check(
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
	statementDisallowUsingTemporaryRule := NewStatementDisallowUsingTemporaryRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{statementDisallowUsingTemporaryRule})

	for _, stmtNode := range root {
		statementDisallowUsingTemporaryRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
