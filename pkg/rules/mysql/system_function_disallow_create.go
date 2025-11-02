package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// SystemFunctionDisallowCreateRule is the ANTLR-based implementation for checking disallowed function creation
type SystemFunctionDisallowCreateRule struct {
	BaseAntlrRule
	text string
}

// NewSystemFunctionDisallowCreateRule creates a new ANTLR-based system function disallow create rule
func NewSystemFunctionDisallowCreateRule(level types.SQLReviewRuleLevel, title string) *SystemFunctionDisallowCreateRule {
	return &SystemFunctionDisallowCreateRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*SystemFunctionDisallowCreateRule) Name() string {
	return "SystemFunctionDisallowCreateRule"
}

// OnEnter is called when entering a parse tree node
func (r *SystemFunctionDisallowCreateRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeCreateFunction:
		r.checkCreateFunction(ctx.(*mysql.CreateFunctionContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*SystemFunctionDisallowCreateRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *SystemFunctionDisallowCreateRule) checkCreateFunction(ctx *mysql.CreateFunctionContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.FunctionName() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DisallowCreateFunction),
			Title:         r.title,
			Content:       fmt.Sprintf("Function is forbidden, but \"%s\" creates", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// SystemFunctionDisallowCreateAdvisor is the advisor using ANTLR parser for system function disallow create checking
type SystemFunctionDisallowCreateAdvisor struct{}

// Check performs the ANTLR-based system function disallow create check
func (a *SystemFunctionDisallowCreateAdvisor) Check(
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
	functionRule := NewSystemFunctionDisallowCreateRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{functionRule})

	for _, stmtNode := range root {
		functionRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
