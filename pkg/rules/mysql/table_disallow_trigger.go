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

// TableDisallowTriggerRule is the ANTLR-based implementation for checking disallowed table triggers
type TableDisallowTriggerRule struct {
	BaseAntlrRule
	text string
}

// NewTableDisallowTriggerRule creates a new ANTLR-based table disallow trigger rule
func NewTableDisallowTriggerRule(level types.SQLReviewRuleLevel, title string) *TableDisallowTriggerRule {
	return &TableDisallowTriggerRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*TableDisallowTriggerRule) Name() string {
	return "TableDisallowTriggerRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableDisallowTriggerRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeCreateTrigger:
		r.checkCreateTrigger(ctx.(*mysql.CreateTriggerContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableDisallowTriggerRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableDisallowTriggerRule) checkCreateTrigger(ctx *mysql.CreateTriggerContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.CreateTableTrigger),
		Title:         r.title,
		Content:       fmt.Sprintf("Trigger is forbidden, but \"%s\" creates", r.text),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
	})
}

// TableDisallowTriggerAdvisor is the advisor using ANTLR parser for table disallow trigger checking
type TableDisallowTriggerAdvisor struct{}

// Check performs the ANTLR-based table disallow trigger check
func (a *TableDisallowTriggerAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	triggerRule := NewTableDisallowTriggerRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{triggerRule})

	for _, stmtNode := range root {
		triggerRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}