package mysql

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// StatementDisallowUsingFilesortRule is the ANTLR-based implementation for checking using filesort
type StatementDisallowUsingFilesortRule struct {
	BaseAntlrRule
}

// NewStatementDisallowUsingFilesortRule creates a new ANTLR-based statement disallow using filesort rule
func NewStatementDisallowUsingFilesortRule(level types.SQLReviewRuleLevel, title string) *StatementDisallowUsingFilesortRule {
	return &StatementDisallowUsingFilesortRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementDisallowUsingFilesortRule) Name() string {
	return "StatementDisallowUsingFilesortRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementDisallowUsingFilesortRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeSelectStatement:
		r.checkSelectStatement(ctx.(*mysql.SelectStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementDisallowUsingFilesortRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementDisallowUsingFilesortRule) checkSelectStatement(ctx *mysql.SelectStatementContext) {
	// For the CLI implementation, we don't have a database connection to run EXPLAIN queries
	// This is a simplified version that reports a warning for any SELECT statement
	// advising users to check for filesort manually
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.StatementHasUsingFilesort),
		Title:         r.title,
		Content:       "Using filesort check required. Please verify that this query does not use filesort by running EXPLAIN and checking the 'Extra' column.",
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
	})
}

// StatementDisallowUsingFilesortAdvisor is the advisor using ANTLR parser for statement disallow using filesort checking
type StatementDisallowUsingFilesortAdvisor struct{}

// Check performs the ANTLR-based statement disallow using filesort check
func (a *StatementDisallowUsingFilesortAdvisor) Check(
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
	statementDisallowUsingFilesortRule := NewStatementDisallowUsingFilesortRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{statementDisallowUsingFilesortRule})

	for _, stmtNode := range root {
		statementDisallowUsingFilesortRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
