package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type StatementDmlDryRunAdvisor struct {
}

func (a *StatementDmlDryRunAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	dmlDryRunRule := NewStatementDmlDryRunRule(ctx, level, string(rule.Type), checkContext.Driver)

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{dmlDryRunRule})

	if checkContext.Driver != nil {
		for _, stmt := range stmtList {
			dmlDryRunRule.SetBaseLine(stmt.BaseLine)
			checker.SetBaseLine(stmt.BaseLine)
			antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
			if dmlDryRunRule.GetExplainCount() >= 100 { // MaximumLintExplainSize
				break
			}
		}
	}

	return checker.GetAdviceList(), nil
}

// StatementDmlDryRunRule checks for DML dry run.
type StatementDmlDryRunRule struct {
	BaseRule
	driver       *sql.DB
	ctx          context.Context
	explainCount int
}

// NewStatementDmlDryRunRule creates a new StatementDmlDryRunRule.
func NewStatementDmlDryRunRule(ctx context.Context, level types.Advice_Status, title string, driver *sql.DB) *StatementDmlDryRunRule {
	return &StatementDmlDryRunRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		driver: driver,
		ctx:    ctx,
	}
}

// Name returns the rule name.
func (*StatementDmlDryRunRule) Name() string {
	return "StatementDmlDryRunRule"
}

// OnEnter is called when entering a parse tree node.
func (r *StatementDmlDryRunRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeUpdateStatement:
		updateCtx, ok := ctx.(*mysql.UpdateStatementContext)
		if !ok {
			return nil
		}
		if mysqlparser.IsTopMySQLRule(&updateCtx.BaseParserRuleContext) {
			r.handleStmt(updateCtx.GetParser().GetTokenStream().GetTextFromRuleContext(updateCtx), updateCtx.GetStart().GetLine())
		}
	case NodeTypeDeleteStatement:
		deleteCtx, ok := ctx.(*mysql.DeleteStatementContext)
		if !ok {
			return nil
		}
		if mysqlparser.IsTopMySQLRule(&deleteCtx.BaseParserRuleContext) {
			r.handleStmt(deleteCtx.GetParser().GetTokenStream().GetTextFromRuleContext(deleteCtx), deleteCtx.GetStart().GetLine())
		}
	case NodeTypeInsertStatement:
		insertCtx, ok := ctx.(*mysql.InsertStatementContext)
		if !ok {
			return nil
		}
		if mysqlparser.IsTopMySQLRule(&insertCtx.BaseParserRuleContext) {
			r.handleStmt(insertCtx.GetParser().GetTokenStream().GetTextFromRuleContext(insertCtx), insertCtx.GetStart().GetLine())
		}
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*StatementDmlDryRunRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

// GetExplainCount returns the explain count.
func (r *StatementDmlDryRunRule) GetExplainCount() int {
	return r.explainCount
}

func (r *StatementDmlDryRunRule) handleStmt(text string, lineNumber int) {
	r.explainCount++
	if _, err := advisor.Query(r.ctx, advisor.QueryContext{}, r.driver, types.Engine_MYSQL, fmt.Sprintf("EXPLAIN %s", text)); err != nil {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.StatementDMLDryRunFailed),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" dry runs failed: %s", text, err.Error()),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + lineNumber),
		})
	}
}