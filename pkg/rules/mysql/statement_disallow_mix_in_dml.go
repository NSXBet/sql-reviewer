package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

type StatementDisallowMixInDMLAdvisor struct{}

func (a *StatementDisallowMixInDMLAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	// Only check when change type is DML
	switch checkContext.ChangeType {
	case types.PlanCheckRunConfig_DML:
		// Continue with the check
	default:
		// Not DML mode, no need to check
		return nil, nil
	}

	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}
	title := string(rule.Type)

	var adviceList []*types.Advice
	for _, stmt := range stmtList {
		checker := &mysqlparser.StatementTypeChecker{}
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)

		if checker.IsDDL {
			adviceList = append(adviceList, &types.Advice{
				Status:        level,
				Code:          int32(types.StatementDisallowMixDDLDML),
				Title:         title,
				Content:       fmt.Sprintf("Data change can only run DML, \"%s\" is not DML", checker.Text),
				StartPosition: ConvertANTLRLineToPosition(stmt.BaseLine),
			})
		}
	}

	return adviceList, nil
}
