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

// StatementInsertMustSpecifyColumnRule is the ANTLR-based implementation for checking INSERT column specification
type StatementInsertMustSpecifyColumnRule struct {
	BaseAntlrRule
	hasSelect bool
	text      string
}

// NewStatementInsertMustSpecifyColumnRule creates a new ANTLR-based statement insert must specify column rule
func NewStatementInsertMustSpecifyColumnRule(level types.SQLReviewRuleLevel, title string) *StatementInsertMustSpecifyColumnRule {
	return &StatementInsertMustSpecifyColumnRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementInsertMustSpecifyColumnRule) Name() string {
	return "StatementInsertMustSpecifyColumnRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementInsertMustSpecifyColumnRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeInsertStatement:
		r.checkInsertStatement(ctx.(*mysql.InsertStatementContext))
	case NodeTypeSelectItemList:
		r.checkSelectItemList(ctx.(*mysql.SelectItemListContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementInsertMustSpecifyColumnRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementInsertMustSpecifyColumnRule) checkInsertStatement(ctx *mysql.InsertStatementContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.InsertQueryExpression() != nil {
		r.hasSelect = true
	}

	if ctx.InsertFromConstructor() == nil {
		return
	}

	if ctx.InsertFromConstructor() != nil && ctx.InsertFromConstructor().Fields() != nil &&
		len(ctx.InsertFromConstructor().Fields().AllInsertIdentifier()) > 0 {
		// has columns.
		return
	}
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.InsertNotSpecifyColumn),
		Title:         r.title,
		Content:       fmt.Sprintf("The INSERT statement must specify columns but \"%s\" does not", r.text),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
	})
}

func (r *StatementInsertMustSpecifyColumnRule) checkSelectItemList(ctx *mysql.SelectItemListContext) {
	if r.hasSelect && ctx.MULT_OPERATOR() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.InsertNotSpecifyColumn),
			Title:         r.title,
			Content:       fmt.Sprintf("The INSERT statement must specify columns but \"%s\" does not", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// StatementInsertMustSpecifyColumnAdvisor is the advisor using ANTLR parser for statement insert must specify column checking
type StatementInsertMustSpecifyColumnAdvisor struct{}

// Check performs the ANTLR-based statement insert must specify column check
func (a *StatementInsertMustSpecifyColumnAdvisor) Check(
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
	insertRule := NewStatementInsertMustSpecifyColumnRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{insertRule})

	for _, stmtNode := range root {
		insertRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
