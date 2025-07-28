package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"github.com/pkg/errors"
)

// StatementMaximumStatementsInTransactionRule is the ANTLR-based implementation for checking maximum statements in transaction
type StatementMaximumStatementsInTransactionRule struct {
	BaseAntlrRule
	limitMaxValue int
	count         int
}

// NewStatementMaximumStatementsInTransactionRule creates a new ANTLR-based statement maximum statements in transaction rule
func NewStatementMaximumStatementsInTransactionRule(
	level types.SQLReviewRuleLevel,
	title string,
	limitMaxValue int,
) *StatementMaximumStatementsInTransactionRule {
	return &StatementMaximumStatementsInTransactionRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		limitMaxValue: limitMaxValue,
	}
}

// Name returns the rule name
func (*StatementMaximumStatementsInTransactionRule) Name() string {
	return "StatementMaximumStatementsInTransactionRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementMaximumStatementsInTransactionRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeSimpleStatement:
		r.checkSimpleStatement(ctx.(*mysql.SimpleStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementMaximumStatementsInTransactionRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementMaximumStatementsInTransactionRule) checkSimpleStatement(ctx *mysql.SimpleStatementContext) {
	r.count++
	// If we exceed the maximum number of statements, report it
	if r.count > r.limitMaxValue {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.StatementMaximumStatementsInTransaction),
			Title:  r.title,
			Content: fmt.Sprintf(
				"Number of statements (%d) exceeds the maximum limit (%d) for a transaction.",
				r.count,
				r.limitMaxValue,
			),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// StatementMaximumStatementsInTransactionAdvisor is the advisor using ANTLR parser for statement maximum statements in transaction checking
type StatementMaximumStatementsInTransactionAdvisor struct{}

// Check performs the ANTLR-based statement maximum statements in transaction check
func (a *StatementMaximumStatementsInTransactionAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	// Create the rule
	statementMaximumStatementsInTransactionRule := NewStatementMaximumStatementsInTransactionRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
		payload.Number,
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{statementMaximumStatementsInTransactionRule})

	for _, stmtNode := range root {
		statementMaximumStatementsInTransactionRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
