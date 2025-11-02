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

// StatementMaximumJoinTableCountRule is the ANTLR-based implementation for checking maximum join table count
type StatementMaximumJoinTableCountRule struct {
	BaseAntlrRule
	text          string
	limitMaxValue int
	count         int
}

// NewStatementMaximumJoinTableCountRule creates a new ANTLR-based statement maximum join table count rule
func NewStatementMaximumJoinTableCountRule(
	level types.SQLReviewRuleLevel,
	title string,
	limitMaxValue int,
) *StatementMaximumJoinTableCountRule {
	return &StatementMaximumJoinTableCountRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		limitMaxValue: limitMaxValue,
	}
}

// Name returns the rule name
func (*StatementMaximumJoinTableCountRule) Name() string {
	return "StatementMaximumJoinTableCountRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementMaximumJoinTableCountRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
			r.count = 0 // Reset count for each query
		}
	case NodeTypeJoinedTable:
		r.checkJoinedTable(ctx.(*mysql.JoinedTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementMaximumJoinTableCountRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementMaximumJoinTableCountRule) checkJoinedTable(ctx *mysql.JoinedTableContext) {
	r.count++
	// The count starts from 0. We count the number of tables in the joins.
	if r.count == r.limitMaxValue {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.StatementMaximumJoinTableCount),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" exceeds the maximum number of joins %d.", r.text, r.limitMaxValue),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// StatementMaximumJoinTableCountAdvisor is the advisor using ANTLR parser for statement maximum join table count checking
type StatementMaximumJoinTableCountAdvisor struct{}

// Check performs the ANTLR-based statement maximum join table count check
func (a *StatementMaximumJoinTableCountAdvisor) Check(
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

	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	// Create the rule
	joinRule := NewStatementMaximumJoinTableCountRule(types.SQLReviewRuleLevel(level), string(rule.Type), payload.Number)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{joinRule})

	for _, stmtNode := range root {
		joinRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
