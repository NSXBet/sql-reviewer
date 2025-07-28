package mysql

import (
	"context"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type StatementRequireLockOptionAdvisor struct {
}

func (a *StatementRequireLockOptionAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	lockRule := NewRequireLockOptionRule(level, string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{lockRule})

	for _, stmt := range stmtList {
		lockRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

// RequireLockOptionRule checks for required LOCK option in ALTER TABLE statements.
type RequireLockOptionRule struct {
	BaseRule
	hasOption             bool
	inAlterTableStatement bool
	text                  string
	line                  int
}

// NewRequireLockOptionRule creates a new RequireLockOptionRule.
func NewRequireLockOptionRule(level types.Advice_Status, title string) *RequireLockOptionRule {
	return &RequireLockOptionRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name.
func (*RequireLockOptionRule) Name() string {
	return "RequireLockOptionRule"
}

// OnEnter is called when entering a parse tree node.
func (r *RequireLockOptionRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeAlterTableActions:
		r.checkAlterTableActions(ctx.(*mysql.AlterTableActionsContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (r *RequireLockOptionRule) OnExit(_ antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeAlterTable {
		if !r.hasOption {
			r.AddAdvice(&types.Advice{
				Status:        r.level,
				Code:          int32(types.StatementNoLockOption),
				Title:         r.title,
				Content:       "ALTER TABLE statement should include LOCK option",
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + r.line),
			})
		}
		r.inAlterTableStatement = false
		r.hasOption = false
	}
	return nil
}

func (r *RequireLockOptionRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	r.inAlterTableStatement = true
	r.text = ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
	r.line = ctx.GetStart().GetLine()
}

func (r *RequireLockOptionRule) checkAlterTableActions(ctx *mysql.AlterTableActionsContext) {
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
		if modifier.AlterLockOption() != nil {
			if modifier.AlterLockOption().Identifier() != nil {
				lockOptionValue := mysqlparser.NormalizeMySQLIdentifier(modifier.AlterLockOption().Identifier())
				// Don't need to check the value of the lock option right now.
				if lockOptionValue != "" {
					r.hasOption = true
				}
			}
		}
	}
}