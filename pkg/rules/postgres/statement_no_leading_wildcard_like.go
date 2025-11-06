package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementNoLeadingWildcardLikeAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleStatementNoLeadingWildcardLike), &StatementNoLeadingWildcardLikeAdvisor{})
}

// StatementNoLeadingWildcardLikeAdvisor is the advisor checking for no leading wildcard LIKE.
type StatementNoLeadingWildcardLikeAdvisor struct{}

// Check checks for no leading wildcard LIKE.
func (*StatementNoLeadingWildcardLikeAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementNoLeadingWildcardLikeChecker{
		level:              level,
		title:              string(checkCtx.Rule.Type),
		statementsText:     checkCtx.Statements,
		reportedStatements: make(map[antlr.ParserRuleContext]bool),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementNoLeadingWildcardLikeChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList         []*types.Advice
	level              types.Advice_Status
	title              string
	statementsText     string
	reportedStatements map[antlr.ParserRuleContext]bool
}

// EnterA_expr_like handles LIKE/ILIKE expressions
func (c *statementNoLeadingWildcardLikeChecker) EnterA_expr_like(ctx *parser.A_expr_likeContext) {
	// Check if this is a LIKE or ILIKE expression (not SIMILAR TO)
	if ctx.LIKE() == nil && ctx.ILIKE() == nil {
		return
	}

	// Get the pattern (second A_expr_qual_op)
	qualOps := ctx.AllA_expr_qual_op()
	if len(qualOps) < 2 {
		return
	}

	pattern := qualOps[1]
	patternStr := extractPatternString(pattern)

	// Check if pattern starts with wildcard
	if patternStr != "" && (strings.HasPrefix(patternStr, "%") || strings.HasPrefix(patternStr, "_")) {
		// Find the top-level statement for reporting
		stmtCtx := findTopLevelStatement(ctx)
		if stmtCtx == nil {
			return
		}

		// Only report once per statement
		if c.reportedStatements[stmtCtx] {
			return
		}
		c.reportedStatements[stmtCtx] = true

		stmtText := extractStatementText(c.statementsText, stmtCtx.GetStart().GetLine(), stmtCtx.GetStop().GetLine())
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(advisor.PostgreSQLStatementNoLeadingWildcardLike),
			Title:   c.title,
			Content: fmt.Sprintf("\"%s\" uses leading wildcard LIKE", stmtText),
			StartPosition: &types.Position{
				Line: int32(stmtCtx.GetStart().GetLine()),
			},
		})
	}
}

// extractPatternString extracts the string literal from a LIKE pattern expression
func extractPatternString(ctx parser.IA_expr_qual_opContext) string {
	if ctx == nil {
		return ""
	}

	// Walk down the expression tree to find the string constant
	var findSconst func(antlr.Tree) parser.ISconstContext
	findSconst = func(node antlr.Tree) parser.ISconstContext {
		if node == nil {
			return nil
		}

		// Check if this is a Sconst context
		if sconst, ok := node.(parser.ISconstContext); ok {
			return sconst
		}

		// Recursively check children
		if prCtx, ok := node.(antlr.ParserRuleContext); ok {
			for i := 0; i < prCtx.GetChildCount(); i++ {
				if result := findSconst(prCtx.GetChild(i)); result != nil {
					return result
				}
			}
		}

		return nil
	}

	sconstCtx := findSconst(ctx)
	if sconstCtx == nil {
		return ""
	}

	// Extract the string value from the Sconst context
	text := sconstCtx.GetText()
	if len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
		return text[1 : len(text)-1]
	}

	return text
}

// findTopLevelStatement finds the top-level statement containing the given context
func findTopLevelStatement(ctx antlr.ParserRuleContext) antlr.ParserRuleContext {
	current := ctx
	for current != nil {
		parent := current.GetParent()
		if isTopLevel(parent) {
			return current
		}
		if prCtx, ok := parent.(antlr.ParserRuleContext); ok {
			current = prCtx
		} else {
			break
		}
	}
	return current
}
