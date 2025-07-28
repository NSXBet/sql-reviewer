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

// SystemViewDisallowCreateRule is the ANTLR-based implementation for checking disallowed view creation
type SystemViewDisallowCreateRule struct {
	BaseAntlrRule
	text string
}

// NewSystemViewDisallowCreateRule creates a new ANTLR-based system view disallow create rule
func NewSystemViewDisallowCreateRule(level types.SQLReviewRuleLevel, title string) *SystemViewDisallowCreateRule {
	return &SystemViewDisallowCreateRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*SystemViewDisallowCreateRule) Name() string {
	return "SystemViewDisallowCreateRule"
}

// OnEnter is called when entering a parse tree node
func (r *SystemViewDisallowCreateRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeCreateView:
		r.checkCreateView(ctx.(*mysql.CreateViewContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*SystemViewDisallowCreateRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *SystemViewDisallowCreateRule) checkCreateView(ctx *mysql.CreateViewContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.ViewName() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DisallowCreateView),
			Title:         r.title,
			Content:       fmt.Sprintf("View is forbidden, but \"%s\" creates", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// SystemViewDisallowCreateAdvisor is the advisor using ANTLR parser for system view disallow create checking
type SystemViewDisallowCreateAdvisor struct{}

// Check performs the ANTLR-based system view disallow create check
func (a *SystemViewDisallowCreateAdvisor) Check(
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
	viewRule := NewSystemViewDisallowCreateRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{viewRule})

	for _, stmtNode := range root {
		viewRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
