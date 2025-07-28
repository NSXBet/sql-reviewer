package mysql

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type StatementMaxExecutionTimeAdvisor struct {
}

func (a *StatementMaxExecutionTimeAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Check if this is MariaDB
	systemVariable := "max_execution_time"
	if rule.Engine == types.Engine_MARIADB {
		systemVariable = "max_statement_time"
	}

	// Create the rule
	maxExecRule := NewMaxExecutionTimeRule(level, string(rule.Type), systemVariable)

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{maxExecRule})

	for _, stmt := range stmtList {
		maxExecRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

// MaxExecutionTimeRule checks for the max execution time using generic checker pattern
type MaxExecutionTimeRule struct {
	BaseRule
	// The system variable name for max execution time.
	// For MySQL, it is `max_execution_time`.
	// For MariaDB, it is `max_statement_time`.
	systemVariable string
	hasSet         bool
}

// NewMaxExecutionTimeRule creates a new MaxExecutionTimeRule.
func NewMaxExecutionTimeRule(level types.Advice_Status, title string, systemVariable string) *MaxExecutionTimeRule {
	return &MaxExecutionTimeRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		systemVariable: systemVariable,
	}
}

// Name returns the rule name.
func (*MaxExecutionTimeRule) Name() string {
	return "MaxExecutionTimeRule"
}

// OnEnter is called when entering a parse tree node.
func (r *MaxExecutionTimeRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeSimpleStatement:
		r.checkSimpleStatement(ctx.(*mysql.SimpleStatementContext))
	case NodeTypeSetStatement:
		r.checkSetStatement(ctx.(*mysql.SetStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*MaxExecutionTimeRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *MaxExecutionTimeRule) checkSimpleStatement(ctx *mysql.SimpleStatementContext) {
	// Skip if we have already found the system variable is set or the statement is a SET statement.
	if r.hasSet || ctx.SetStatement() != nil {
		return
	}

	// The set max execution time statement should be the first statement in the SQL.
	// Otherwise, we will always set the advice (but only once).
	if len(r.adviceList) == 0 {
		r.setAdvice()
	}
}

func (r *MaxExecutionTimeRule) checkSetStatement(ctx *mysql.SetStatementContext) {
	startOptionValueList := ctx.StartOptionValueList()
	if ctx.StartOptionValueList() == nil {
		return
	}

	variable, value := "", ""
	optionValueList := startOptionValueList.StartOptionValueListFollowingOptionType()
	if optionValueList != nil {
		tmp := optionValueList.OptionValueFollowingOptionType()
		if tmp != nil {
			if tmp.InternalVariableName() != nil && tmp.SetExprOrDefault() != nil {
				variable, value = tmp.InternalVariableName().GetText(), tmp.SetExprOrDefault().GetText()
			}
		}
	}
	optionValueNoOptionType := startOptionValueList.OptionValueNoOptionType()
	if optionValueNoOptionType != nil {
		if optionValueNoOptionType.InternalVariableName() != nil && optionValueNoOptionType.SetExprOrDefault() != nil {
			variable, value = optionValueNoOptionType.InternalVariableName().GetText(), optionValueNoOptionType.SetExprOrDefault().GetText()
		}
	}
	_, err := strconv.Atoi(value)
	if strings.ToLower(variable) == r.systemVariable && err == nil {
		r.hasSet = true
	}
}

func (r *MaxExecutionTimeRule) setAdvice() {
	r.adviceList = append(r.adviceList, &types.Advice{
		Status:  r.level,
		Code:    int32(types.StatementNoMaxExecutionTime),
		Title:   r.title,
		Content: fmt.Sprintf("The %s is not set", r.systemVariable),
	})
}