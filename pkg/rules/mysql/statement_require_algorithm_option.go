package mysql

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

type StatementRequireAlgorithmOptionAdvisor struct{}

func (a *StatementRequireAlgorithmOptionAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	algorithmRule := NewRequireAlgorithmOptionRule(level, string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{algorithmRule})

	for _, stmt := range stmtList {
		algorithmRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

// RequireAlgorithmOptionRule checks for required ALGORITHM option in ALTER TABLE statements.
type RequireAlgorithmOptionRule struct {
	BaseRule
	hasOption             bool
	inAlterTableStatement bool
	text                  string
	line                  int
}

// NewRequireAlgorithmOptionRule creates a new RequireAlgorithmOptionRule.
func NewRequireAlgorithmOptionRule(level types.Advice_Status, title string) *RequireAlgorithmOptionRule {
	return &RequireAlgorithmOptionRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name.
func (*RequireAlgorithmOptionRule) Name() string {
	return "RequireAlgorithmOptionRule"
}

// OnEnter is called when entering a parse tree node.
func (r *RequireAlgorithmOptionRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeAlterTableActions:
		r.checkAlterTableActions(ctx.(*mysql.AlterTableActionsContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (r *RequireAlgorithmOptionRule) OnExit(_ antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeAlterTable {
		if !r.hasOption {
			r.AddAdvice(&types.Advice{
				Status:        r.level,
				Code:          int32(types.StatementNoAlgorithmOption),
				Title:         r.title,
				Content:       "ALTER TABLE statement should include ALGORITHM option",
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + r.line),
			})
		}
		r.inAlterTableStatement = false
		r.hasOption = false
	}
	return nil
}

func (r *RequireAlgorithmOptionRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	r.inAlterTableStatement = true
	r.text = ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
	r.line = ctx.GetStart().GetLine()
}

func (r *RequireAlgorithmOptionRule) checkAlterTableActions(ctx *mysql.AlterTableActionsContext) {
	if !r.inAlterTableStatement {
		return
	}

	modifierList := []mysql.IAlterCommandsModifierContext{}
	if ctx.AlterCommandsModifierList() != nil {
		modifierList = append(modifierList, ctx.AlterCommandsModifierList().AllAlterCommandsModifier()...)
	}
	if ctx.AlterCommandList() != nil {
		if ctx.AlterCommandList().AlterCommandsModifierList() != nil {
			modifierList = append(modifierList, ctx.AlterCommandList().AlterCommandsModifierList().AllAlterCommandsModifier()...)
		}
		if ctx.AlterCommandList().AlterList() != nil {
			modifierList = append(modifierList, ctx.AlterCommandList().AlterList().AllAlterCommandsModifier()...)
		}
	}
	for _, modifier := range modifierList {
		if modifier.AlterAlgorithmOption() != nil {
			if modifier.AlterAlgorithmOption().Identifier() != nil {
				algorithmOptionValue := mysqlparser.NormalizeMySQLIdentifier(modifier.AlterAlgorithmOption().Identifier())
				// Don't need to check the value of the algorithm option right now.
				if algorithmOptionValue != "" {
					r.hasOption = true
				}
			}
		}
	}
}
