package mysql

import (
	"context"
	"regexp"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

const implicitCommitDDLMixMessage = "MySQL implicitly commits this statement. Split it into a dedicated single-statement change; cancellation and timeout cannot transactionally roll it back."

var (
	mysqlCommentPattern          = regexp.MustCompile(`(?s)/\*.*?\*/|--[^\r\n]*|#[^\r\n]*`)
	mysqlAutocommitEnablePattern = regexp.MustCompile(
		`^(?:SET(?:SESSION)?AUTOCOMMIT|SET@@SESSION\.AUTOCOMMIT)(?::=|=)1;?(?:<EOF>)?$`,
	)
)

type StatementMySQLDisallowImplicitCommitDDLMixAdvisor struct{}

func (*StatementMySQLDisallowImplicitCommitDDLMixAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	stmtList, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}
	if len(stmtList) < 2 {
		return nil, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	var adviceList []*types.Advice
	for _, stmt := range stmtList {
		if !isMySQLImplicitCommitDDL(stmt.Tree) {
			continue
		}
		adviceList = append(adviceList, &types.Advice{
			Status:        level,
			Code:          int32(advisor.MySQLStatementImplicitCommitDDLMix),
			Title:         string(rule.Type),
			Content:       implicitCommitDDLMixMessage,
			StartPosition: ConvertANTLRLineToPosition(stmt.BaseLine),
		})
	}
	return adviceList, nil
}

func isMySQLImplicitCommitDDL(tree antlr.Tree) bool {
	checker := &mysqlparser.StatementTypeChecker{}
	antlr.ParseTreeWalkerDefault.Walk(checker, tree)
	if checker.IsDDL {
		return true
	}

	statement := strings.ToUpper(mysqlCommentPattern.ReplaceAllString(tree.(antlr.ParseTree).GetText(), ""))
	return strings.HasPrefix(statement, "LOCKTABLES") ||
		strings.HasPrefix(statement, "UNLOCKTABLES") ||
		mysqlAutocommitEnablePattern.MatchString(statement)
}
