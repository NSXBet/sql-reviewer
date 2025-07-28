package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type StatementDisallowMixInDDLAdvisor struct {
}

func (a *StatementDisallowMixInDDLAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	// Only check when change type is DDL
	switch checkContext.ChangeType {
	case types.PlanCheckRunConfig_DDL, types.PlanCheckRunConfig_SDL:
		// Continue with the check
	default:
		// Not DDL mode, no need to check
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

		if checker.IsDML {
			adviceList = append(adviceList, &types.Advice{
				Status:        level,
				Code:          int32(types.StatementDisallowMixDDLDML),
				Title:         title,
				Content:       fmt.Sprintf("Alter schema can only run DDL, \"%s\" is not DDL", checker.Text),
				StartPosition: ConvertANTLRLineToPosition(stmt.BaseLine),
			})
		}
	}

	return adviceList, nil
}