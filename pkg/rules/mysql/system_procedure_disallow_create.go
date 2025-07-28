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

// SystemProcedureDisallowCreateRule is the ANTLR-based implementation for checking disallowed procedure creation
type SystemProcedureDisallowCreateRule struct {
	BaseAntlrRule
	text string
}

// NewSystemProcedureDisallowCreateRule creates a new ANTLR-based system procedure disallow create rule
func NewSystemProcedureDisallowCreateRule(level types.SQLReviewRuleLevel, title string) *SystemProcedureDisallowCreateRule {
	return &SystemProcedureDisallowCreateRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*SystemProcedureDisallowCreateRule) Name() string {
	return "SystemProcedureDisallowCreateRule"
}

// OnEnter is called when entering a parse tree node
func (r *SystemProcedureDisallowCreateRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeCreateProcedure:
		r.checkCreateProcedure(ctx.(*mysql.CreateProcedureContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*SystemProcedureDisallowCreateRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *SystemProcedureDisallowCreateRule) checkCreateProcedure(ctx *mysql.CreateProcedureContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.ProcedureName() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DisallowCreateProcedure),
			Title:         r.title,
			Content:       fmt.Sprintf("Procedure is forbidden, but \"%s\" creates", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// SystemProcedureDisallowCreateAdvisor is the advisor using ANTLR parser for system procedure disallow create checking
type SystemProcedureDisallowCreateAdvisor struct{}

// Check performs the ANTLR-based system procedure disallow create check
func (a *SystemProcedureDisallowCreateAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	procedureRule := NewSystemProcedureDisallowCreateRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{procedureRule})

	for _, stmtNode := range root {
		procedureRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}