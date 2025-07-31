package mysql

import (
	"context"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// StatementJoinStrictColumnAttrsRule is the ANTLR-based implementation for checking join column attributes
type StatementJoinStrictColumnAttrsRule struct {
	BaseAntlrRule
	isSelect       bool
	isInFromClause bool
	joinFound      bool
}

// NewStatementJoinStrictColumnAttrsRule creates a new ANTLR-based statement join strict column attrs rule
func NewStatementJoinStrictColumnAttrsRule(level types.SQLReviewRuleLevel, title string) *StatementJoinStrictColumnAttrsRule {
	return &StatementJoinStrictColumnAttrsRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementJoinStrictColumnAttrsRule) Name() string {
	return "StatementJoinStrictColumnAttrsRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementJoinStrictColumnAttrsRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeSelectStatement:
		r.isSelect = true
		r.joinFound = false
	case NodeTypeFromClause:
		r.isInFromClause = true
		r.handleFromClause(ctx.(*mysql.FromClauseContext))
	case NodeTypePrimaryExprCompare:
		if r.isSelect && r.isInFromClause {
			r.handlePrimaryExprCompare(ctx.(*mysql.PrimaryExprCompareContext))
		}
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementJoinStrictColumnAttrsRule) OnExit(ctx antlr.ParserRuleContext, nodeType string) error {
	return nil
}

func (r *StatementJoinStrictColumnAttrsRule) handleFromClause(ctx *mysql.FromClauseContext) {
	if !r.isSelect || ctx.TableReferenceList() == nil {
		return
	}

	tableRefs := ctx.TableReferenceList().AllTableReference()

	// Check if there are JOIN operations
	for _, tableRef := range tableRefs {
		if tableRef.AllJoinedTable() != nil && len(tableRef.AllJoinedTable()) > 0 {
			r.joinFound = true
			break
		}
	}

	// Check for comma-separated multiple tables (old-style joins)
	if len(tableRefs) > 1 {
		r.joinFound = true
	}
}

func (r *StatementJoinStrictColumnAttrsRule) handlePrimaryExprCompare(ctx *mysql.PrimaryExprCompareContext) {
	if !r.joinFound {
		return
	}

	compOp := ctx.CompOp()
	// We only check for equal for now.
	if compOp == nil || compOp.EQUAL_OPERATOR() == nil {
		return
	}
	if ctx.BoolPri() == nil || ctx.Predicate() == nil {
		return
	}

	leftText := ctx.BoolPri().GetText()
	rightText := ctx.Predicate().GetText()

	// Check if both sides are column references (table.column format)
	if isColumnReference(leftText) && isColumnReference(rightText) {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.StatementJoinColumnAttrsNotMatch),
			Title:         r.title,
			Content:       "Join column attributes check required. Please verify that joined columns have matching types, character sets, and collations.",
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func isColumnReference(text string) bool {
	elements := strings.Split(text, ".")
	return len(elements) == 2
}

// StatementJoinStrictColumnAttrsAdvisor is the advisor using ANTLR parser for statement join strict column attrs checking
type StatementJoinStrictColumnAttrsAdvisor struct{}

// Check performs the ANTLR-based statement join strict column attrs check
func (a *StatementJoinStrictColumnAttrsAdvisor) Check(
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
	statementJoinStrictColumnAttrsRule := NewStatementJoinStrictColumnAttrsRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{statementJoinStrictColumnAttrsRule})

	for _, stmtNode := range root {
		statementJoinStrictColumnAttrsRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
