package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

type StatementAffectedRowLimitAdvisor struct{}

func (a *StatementAffectedRowLimitAdvisor) Check(
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

	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	// Create the rule
	affectedRowRule := NewStatementAffectedRowLimitRule(ctx, level, string(rule.Type), payload.Number, checkContext.Driver)

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{affectedRowRule})

	if checkContext.Driver != nil {
		for _, stmt := range stmtList {
			affectedRowRule.SetBaseLine(stmt.BaseLine)
			checker.SetBaseLine(stmt.BaseLine)
			antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
			if affectedRowRule.explainCount >= 100 { // MaximumLintExplainSize
				break
			}
		}
	}

	return checker.GetAdviceList(), nil
}

// StatementAffectedRowLimitRule checks for UPDATE/DELETE affected row limit.
type StatementAffectedRowLimitRule struct {
	BaseRule
	text         string
	maxRow       int
	driver       *sql.DB
	ctx          context.Context
	explainCount int
}

// NewStatementAffectedRowLimitRule creates a new StatementAffectedRowLimitRule.
func NewStatementAffectedRowLimitRule(
	ctx context.Context,
	level types.Advice_Status,
	title string,
	maxRow int,
	driver *sql.DB,
) *StatementAffectedRowLimitRule {
	return &StatementAffectedRowLimitRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		maxRow: maxRow,
		driver: driver,
		ctx:    ctx,
	}
}

// Name returns the rule name.
func (*StatementAffectedRowLimitRule) Name() string {
	return "StatementAffectedRowLimitRule"
}

// OnEnter is called when entering a parse tree node.
func (r *StatementAffectedRowLimitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeUpdateStatement:
		if mysqlparser.IsTopMySQLRule(&ctx.(*mysql.UpdateStatementContext).BaseParserRuleContext) {
			r.handleStmt(ctx.GetStart().GetLine())
		}
	case NodeTypeDeleteStatement:
		if mysqlparser.IsTopMySQLRule(&ctx.(*mysql.DeleteStatementContext).BaseParserRuleContext) {
			r.handleStmt(ctx.GetStart().GetLine())
		}
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*StatementAffectedRowLimitRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementAffectedRowLimitRule) handleStmt(lineNumber int) {
	lineNumber += r.baseLine
	r.explainCount++

	if r.driver == nil {
		// No database connection available, cannot run EXPLAIN
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.StatementAffectedRowExceedsLimit),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" cannot be checked without database connection", r.text),
			StartPosition: ConvertANTLRLineToPosition(lineNumber),
		})
		return
	}

	res, err := advisor.Query(r.ctx, advisor.QueryContext{}, r.driver, types.Engine_MYSQL, fmt.Sprintf("EXPLAIN %s", r.text))
	if err != nil {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.StatementAffectedRowExceedsLimit),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" dry runs failed: %s", r.text, err.Error()),
			StartPosition: ConvertANTLRLineToPosition(lineNumber),
		})
	} else {
		rowCount, err := getRows(res)
		if err != nil {
			r.AddAdvice(&types.Advice{
				Status:        r.level,
				Code:          int32(types.Internal),
				Title:         r.title,
				Content:       fmt.Sprintf("failed to get row count for \"%s\": %s", r.text, err.Error()),
				StartPosition: ConvertANTLRLineToPosition(lineNumber),
			})
		} else if rowCount > int64(r.maxRow) {
			r.AddAdvice(&types.Advice{
				Status:        r.level,
				Code:          int32(types.StatementAffectedRowExceedsLimit),
				Title:         r.title,
				Content:       fmt.Sprintf("\"%s\" affected %d rows (estimated). The count exceeds %d.", r.text, rowCount, r.maxRow),
				StartPosition: ConvertANTLRLineToPosition(lineNumber),
			})
		}
	}
}

func getRows(res []any) (int64, error) {
	// the res struct is []any{columnName, columnTable, rowDataList}
	if len(res) != 3 {
		return 0, fmt.Errorf("expected 3 but got %d", len(res))
	}
	columns, ok := res[0].([]string)
	if !ok {
		return 0, fmt.Errorf("expected []string but got %T", res[0])
	}
	rowList, ok := res[2].([]any)
	if !ok {
		return 0, fmt.Errorf("expected []any but got %T", res[2])
	}
	if len(rowList) < 1 {
		return 0, fmt.Errorf("not found any data")
	}

	// MySQL EXPLAIN statement result has 12 columns.
	// the column 9 is the data 'rows'.
	// the first not-NULL value of column 9 is the affected rows count.
	rowsIndex, err := getColumnIndex(columns, "rows")
	if err != nil {
		return 0, fmt.Errorf("failed to find rows column")
	}

	for _, rowAny := range rowList {
		row, ok := rowAny.([]any)
		if !ok {
			return 0, fmt.Errorf("expected []any but got %T", row)
		}

		switch col := row[rowsIndex].(type) {
		case int:
			return int64(col), nil
		case int32:
			return int64(col), nil
		case int64:
			return col, nil
		case string:
			v, err := strconv.ParseInt(col, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("expected int or int64 but got string(%s)", col)
			}
			return v, nil
		default:
			continue
		}
	}

	return 0, nil
}

func getColumnIndex(columns []string, columnName string) (int, error) {
	for i, col := range columns {
		if col == columnName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("column %s not found", columnName)
}
