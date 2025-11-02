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

// StatementWhereNoEqualNullRule is the ANTLR-based implementation for checking equal NULL in WHERE clause
type StatementWhereNoEqualNullRule struct {
	BaseAntlrRule
	text     string
	isSelect bool
}

// NewStatementWhereNoEqualNullRule creates a new ANTLR-based where no equal null rule
func NewStatementWhereNoEqualNullRule(level types.SQLReviewRuleLevel, title string) *StatementWhereNoEqualNullRule {
	return &StatementWhereNoEqualNullRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementWhereNoEqualNullRule) Name() string {
	return "StatementWhereNoEqualNullRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementWhereNoEqualNullRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeSelectStatement:
		r.isSelect = true
	case NodeTypePrimaryExprCompare:
		r.checkPrimaryExprCompare(ctx.(*mysql.PrimaryExprCompareContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (r *StatementWhereNoEqualNullRule) OnExit(_ antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeSelectStatement {
		r.isSelect = false
	}
	return nil
}

func (r *StatementWhereNoEqualNullRule) checkPrimaryExprCompare(ctx *mysql.PrimaryExprCompareContext) {
	if !r.isSelect {
		return
	}

	compOp := ctx.CompOp()
	// We only check for equal and not equal.
	if compOp == nil || (compOp.EQUAL_OPERATOR() == nil && compOp.NOT_EQUAL_OPERATOR() == nil) {
		return
	}
	if ctx.Predicate() != nil && ctx.Predicate().GetText() == "NULL" {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.StatementWhereNoEqualNull),
			Title:         r.title,
			Content:       fmt.Sprintf("WHERE clause contains equal null: %s", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// StatementWhereNoEqualNullAdvisor is the advisor using ANTLR parser for where no equal null checking
type StatementWhereNoEqualNullAdvisor struct{}

// Check performs the ANTLR-based where no equal null check
func (a *StatementWhereNoEqualNullAdvisor) Check(
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
	whereRule := NewStatementWhereNoEqualNullRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{whereRule})

	for _, stmtNode := range root {
		whereRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
