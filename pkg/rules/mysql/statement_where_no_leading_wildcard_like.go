package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// StatementWhereNoLeadingWildcardLikeRule is the ANTLR-based implementation for checking leading wildcard LIKE clauses
type StatementWhereNoLeadingWildcardLikeRule struct {
	BaseAntlrRule
	text string
}

// NewStatementWhereNoLeadingWildcardLikeRule creates a new ANTLR-based statement where no leading wildcard like rule
func NewStatementWhereNoLeadingWildcardLikeRule(
	level types.SQLReviewRuleLevel,
	title string,
) *StatementWhereNoLeadingWildcardLikeRule {
	return &StatementWhereNoLeadingWildcardLikeRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementWhereNoLeadingWildcardLikeRule) Name() string {
	return "StatementWhereNoLeadingWildcardLikeRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementWhereNoLeadingWildcardLikeRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypePredicateExprLike:
		r.checkPredicateExprLike(ctx.(*mysql.PredicateExprLikeContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementWhereNoLeadingWildcardLikeRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementWhereNoLeadingWildcardLikeRule) checkPredicateExprLike(ctx *mysql.PredicateExprLikeContext) {
	if ctx.LIKE_SYMBOL() == nil {
		return
	}

	for _, expr := range ctx.AllSimpleExpr() {
		pattern := expr.GetText()
		if (strings.HasPrefix(pattern, "'%") && strings.HasSuffix(pattern, "'")) ||
			(strings.HasPrefix(pattern, "\"%") && strings.HasSuffix(pattern, "\"")) {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.StatementLeadingWildcardLike),
				Title:         r.title,
				Content:       fmt.Sprintf("\"%s\" uses leading wildcard LIKE", r.text),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// StatementWhereNoLeadingWildcardLikeAdvisor is the advisor using ANTLR parser for statement where no leading wildcard like checking
type StatementWhereNoLeadingWildcardLikeAdvisor struct{}

// Check performs the ANTLR-based statement where no leading wildcard like check
func (a *StatementWhereNoLeadingWildcardLikeAdvisor) Check(
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
	likeRule := NewStatementWhereNoLeadingWildcardLikeRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{likeRule})

	for _, stmtNode := range root {
		likeRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
