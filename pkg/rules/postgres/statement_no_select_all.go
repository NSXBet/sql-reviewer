package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementNoSelectAllAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleStatementNoSelectAll), &StatementNoSelectAllAdvisor{})
}

// StatementNoSelectAllAdvisor is the advisor checking for no "SELECT *".
type StatementNoSelectAllAdvisor struct{}

// Check checks for no "SELECT *".
func (*StatementNoSelectAllAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &statementNoSelectAllChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		statementsText: checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type statementNoSelectAllChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

// EnterSimple_select_pramary checks for SELECT * in simple select statements
func (c *statementNoSelectAllChecker) EnterSimple_select_pramary(ctx *parser.Simple_select_pramaryContext) {
	// Check if this is a SELECT statement with target list
	if ctx.SELECT() == nil {
		return
	}

	// Check the target list for * (asterisk)
	if ctx.Opt_target_list() != nil && ctx.Opt_target_list().Target_list() != nil {
		targetList := ctx.Opt_target_list().Target_list()
		allTargets := targetList.AllTarget_el()

		for _, target := range allTargets {
			// Check if target is a Target_star (SELECT *)
			if _, ok := target.(*parser.Target_starContext); ok {
				// Find the top-level statement context to get the full statement text
				var stmtCtx antlr.ParserRuleContext
				parent := ctx.GetParent()
				for parent != nil {
					// Look for top-level statement contexts
					switch p := parent.(type) {
					case *parser.SelectstmtContext:
						if isTopLevel(p.GetParent()) {
							stmtCtx = p
						}
					case *parser.InsertstmtContext:
						if isTopLevel(p.GetParent()) {
							stmtCtx = p
						}
					}
					if stmtCtx != nil {
						break
					}
					parent = parent.GetParent()
				}

				// If we found a top-level statement, extract its text
				var stmtText string
				var line int
				if stmtCtx != nil {
					stmtText = extractStatementText(c.statementsText, stmtCtx.GetStart().GetLine(), stmtCtx.GetStop().GetLine())
					line = stmtCtx.GetStart().GetLine()
				} else {
					// Fallback to the simple_select context
					stmtText = extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
					line = ctx.GetStart().GetLine()
				}

				c.adviceList = append(c.adviceList, &types.Advice{
					Status:  c.level,
					Code:    int32(types.StatementSelectAll),
					Title:   c.title,
					Content: fmt.Sprintf("\"%s\" uses SELECT all", stmtText),
					StartPosition: &types.Position{
						Line: int32(line),
					},
				})
				return
			}
		}
	}
}
