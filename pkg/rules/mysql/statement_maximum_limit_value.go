package mysql

import (
	"context"
	"fmt"
	"strconv"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// StatementMaximumLimitValueRule is the ANTLR-based implementation for checking maximum LIMIT values
type StatementMaximumLimitValueRule struct {
	BaseAntlrRule
	text          string
	isSelect      bool
	limitMaxValue int
}

// NewStatementMaximumLimitValueRule creates a new ANTLR-based statement maximum limit value rule
func NewStatementMaximumLimitValueRule(
	level types.SQLReviewRuleLevel,
	title string,
	limitMaxValue int,
) *StatementMaximumLimitValueRule {
	return &StatementMaximumLimitValueRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		limitMaxValue: limitMaxValue,
	}
}

// Name returns the rule name
func (*StatementMaximumLimitValueRule) Name() string {
	return "StatementMaximumLimitValueRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementMaximumLimitValueRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		if queryCtx, ok := ctx.(*mysql.QueryContext); ok {
			r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		}
	case NodeTypeSelectStatement:
		if mysqlparser.IsTopMySQLRule(&ctx.(*mysql.SelectStatementContext).BaseParserRuleContext) {
			r.isSelect = true
		}
	case NodeTypeLimitClause:
		r.checkLimitClause(ctx.(*mysql.LimitClauseContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (r *StatementMaximumLimitValueRule) OnExit(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeSelectStatement {
		if mysqlparser.IsTopMySQLRule(&ctx.(*mysql.SelectStatementContext).BaseParserRuleContext) {
			r.isSelect = false
		}
	}
	return nil
}

func (r *StatementMaximumLimitValueRule) checkLimitClause(ctx *mysql.LimitClauseContext) {
	if !r.isSelect {
		return
	}
	if ctx.LIMIT_SYMBOL() == nil {
		return
	}

	limitOptions := ctx.LimitOptions()
	for _, limitOption := range limitOptions.AllLimitOption() {
		limitValue, err := strconv.Atoi(limitOption.GetText())
		if err != nil {
			// Ignore invalid limit value and continue.
			continue
		}

		if limitValue > r.limitMaxValue {
			r.AddAdvice(&types.Advice{
				Status: types.Advice_Status(r.level),
				Code:   int32(types.StatementExceedMaximumLimitValue),
				Title:  r.title,
				Content: fmt.Sprintf(
					"The limit value %d exceeds the maximum allowed value %d",
					limitValue,
					r.limitMaxValue,
				),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// StatementMaximumLimitValueAdvisor is the advisor using ANTLR parser for statement maximum limit value checking
type StatementMaximumLimitValueAdvisor struct{}

// Check performs the ANTLR-based statement maximum limit value check
func (a *StatementMaximumLimitValueAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.Context,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Parse the parameter from rule payload
	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	limitMaxValue := int(payload.Number)

	// Create the rule
	limitRule := NewStatementMaximumLimitValueRule(types.SQLReviewRuleLevel(level), string(rule.Type), limitMaxValue)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{limitRule})

	for _, stmtNode := range root {
		limitRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
