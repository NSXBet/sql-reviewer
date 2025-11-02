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

const RandFn = "rand()"

// StatementInsertDisallowOrderByRandRule is the ANTLR-based implementation for checking INSERT statements with ORDER BY RAND
type StatementInsertDisallowOrderByRandRule struct {
	BaseAntlrRule
	isInsertStmt bool
	text         string
}

// NewStatementInsertDisallowOrderByRandRule creates a new ANTLR-based statement insert disallow order by rand rule
func NewStatementInsertDisallowOrderByRandRule(
	level types.SQLReviewRuleLevel,
	title string,
) *StatementInsertDisallowOrderByRandRule {
	return &StatementInsertDisallowOrderByRandRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementInsertDisallowOrderByRandRule) Name() string {
	return "StatementInsertDisallowOrderByRandRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementInsertDisallowOrderByRandRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeInsertStatement:
		if insertCtx, ok := ctx.(*mysql.InsertStatementContext); ok {
			if mysqlparser.IsTopMySQLRule(&insertCtx.BaseParserRuleContext) {
				if insertCtx.InsertQueryExpression() != nil {
					r.isInsertStmt = true
				}
			}
		}
	case NodeTypeQueryExpression:
		r.checkQueryExpression(ctx.(*mysql.QueryExpressionContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (r *StatementInsertDisallowOrderByRandRule) OnExit(_ antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeInsertStatement {
		r.isInsertStmt = false
	}
	return nil
}

func (r *StatementInsertDisallowOrderByRandRule) checkQueryExpression(ctx *mysql.QueryExpressionContext) {
	if !r.isInsertStmt {
		return
	}

	if ctx.OrderClause() == nil || ctx.OrderClause().OrderList() == nil {
		return
	}

	for _, expr := range ctx.OrderClause().OrderList().AllOrderExpression() {
		text := expr.GetText()
		if strings.EqualFold(text, RandFn) {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.InsertUseOrderByRand),
				Title:         r.title,
				Content:       fmt.Sprintf("\"%s\" uses ORDER BY RAND in the INSERT statement", r.text),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// StatementInsertDisallowOrderByRandAdvisor is the advisor using ANTLR parser for statement insert disallow order by rand checking
type StatementInsertDisallowOrderByRandAdvisor struct{}

// Check performs the ANTLR-based statement insert disallow order by rand check
func (a *StatementInsertDisallowOrderByRandAdvisor) Check(
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
	insertRule := NewStatementInsertDisallowOrderByRandRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{insertRule})

	for _, stmtNode := range root {
		insertRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
