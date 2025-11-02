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

// StatementDisallowCommitRule is the ANTLR-based implementation for checking disallowed commit statements
type StatementDisallowCommitRule struct {
	BaseAntlrRule
	text string
}

// NewStatementDisallowCommitRule creates a new ANTLR-based statement disallow commit rule
func NewStatementDisallowCommitRule(level types.SQLReviewRuleLevel, title string) *StatementDisallowCommitRule {
	return &StatementDisallowCommitRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementDisallowCommitRule) Name() string {
	return "StatementDisallowCommitRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementDisallowCommitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeTransactionStatement:
		r.checkTransactionStatement(ctx.(*mysql.TransactionStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementDisallowCommitRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementDisallowCommitRule) checkTransactionStatement(ctx *mysql.TransactionStatementContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.COMMIT_SYMBOL() == nil {
		return
	}

	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.StatementDisallowCommit),
		Title:         r.title,
		Content:       fmt.Sprintf("Commit is not allowed, related statement: \"%s\"", r.text),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
	})
}

// StatementDisallowCommitAdvisor is the advisor using ANTLR parser for statement disallow commit checking
type StatementDisallowCommitAdvisor struct{}

// Check performs the ANTLR-based statement disallow commit check
func (a *StatementDisallowCommitAdvisor) Check(
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
	commitRule := NewStatementDisallowCommitRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{commitRule})

	for _, stmtNode := range root {
		commitRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
