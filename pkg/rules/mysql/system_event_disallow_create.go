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

// SystemEventDisallowCreateRule is the ANTLR-based implementation for checking disallowed event creation
type SystemEventDisallowCreateRule struct {
	BaseAntlrRule
	text string
}

// NewSystemEventDisallowCreateRule creates a new ANTLR-based system event disallow create rule
func NewSystemEventDisallowCreateRule(level types.SQLReviewRuleLevel, title string) *SystemEventDisallowCreateRule {
	return &SystemEventDisallowCreateRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*SystemEventDisallowCreateRule) Name() string {
	return "SystemEventDisallowCreateRule"
}

// OnEnter is called when entering a parse tree node
func (r *SystemEventDisallowCreateRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeCreateEvent:
		r.checkCreateEvent(ctx.(*mysql.CreateEventContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*SystemEventDisallowCreateRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *SystemEventDisallowCreateRule) checkCreateEvent(ctx *mysql.CreateEventContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.EventName() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DisallowCreateEvent),
			Title:         r.title,
			Content:       fmt.Sprintf("Event is forbidden, but \"%s\" creates", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// SystemEventDisallowCreateAdvisor is the advisor using ANTLR parser for system event disallow create checking
type SystemEventDisallowCreateAdvisor struct{}

// Check performs the ANTLR-based system event disallow create check
func (a *SystemEventDisallowCreateAdvisor) Check(
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
	eventRule := NewSystemEventDisallowCreateRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{eventRule})

	for _, stmtNode := range root {
		eventRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
