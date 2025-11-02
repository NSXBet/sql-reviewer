package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type StatementInsertRowLimitAdvisor struct{}

func (a *StatementInsertRowLimitAdvisor) Check(
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
	insertRule := NewInsertRowLimitRule(ctx, level, string(rule.Type), payload.Number, checkContext.Driver)

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{insertRule})

	for _, stmt := range stmtList {
		insertRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
		if insertRule.GetExplainCount() >= 100 { // MaximumLintExplainSize
			break
		}
	}

	return checker.GetAdviceList(), nil
}

// InsertRowLimitRule checks for insert row limit.
type InsertRowLimitRule struct {
	BaseRule
	text         string
	line         int
	maxRow       int
	driver       *sql.DB
	ctx          context.Context
	explainCount int
}

// NewInsertRowLimitRule creates a new InsertRowLimitRule.
func NewInsertRowLimitRule(
	ctx context.Context,
	level types.Advice_Status,
	title string,
	maxRow int,
	driver *sql.DB,
) *InsertRowLimitRule {
	return &InsertRowLimitRule{
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
func (*InsertRowLimitRule) Name() string {
	return "InsertRowLimitRule"
}

// OnEnter is called when entering a parse tree node.
func (r *InsertRowLimitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeInsertStatement:
		r.checkInsertStatement(ctx.(*mysql.InsertStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*InsertRowLimitRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

// GetExplainCount returns the explain count.
func (r *InsertRowLimitRule) GetExplainCount() int {
	return r.explainCount
}

func (r *InsertRowLimitRule) checkInsertStatement(ctx *mysql.InsertStatementContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	r.line = r.baseLine + ctx.GetStart().GetLine()
	if ctx.InsertQueryExpression() != nil {
		r.handleInsertQueryExpression(ctx.InsertQueryExpression())
	}
	r.handleNoInsertQueryExpression(ctx)
}

func (r *InsertRowLimitRule) handleInsertQueryExpression(ctx mysql.IInsertQueryExpressionContext) {
	if r.driver == nil || ctx == nil {
		return
	}

	r.explainCount++
	res, err := advisor.Query(r.ctx, advisor.QueryContext{}, r.driver, types.Engine_MYSQL, fmt.Sprintf("EXPLAIN %s", r.text))
	if err != nil {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.InsertTooManyRows),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" dry runs failed: %s", r.text, err.Error()),
			StartPosition: ConvertANTLRLineToPosition(r.line),
		})
		return
	}
	rowCount, err := getInsertRows(res)
	if err != nil {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.Internal),
			Title:         r.title,
			Content:       fmt.Sprintf("failed to get row count for \"%s\": %s", r.text, err.Error()),
			StartPosition: ConvertANTLRLineToPosition(r.line),
		})
	} else if rowCount > int64(r.maxRow) {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.InsertTooManyRows),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" inserts %d rows. The count exceeds %d.", r.text, rowCount, r.maxRow),
			StartPosition: ConvertANTLRLineToPosition(r.line),
		})
	}
}

func (r *InsertRowLimitRule) handleNoInsertQueryExpression(ctx mysql.IInsertStatementContext) {
	if ctx.InsertFromConstructor() == nil {
		return
	}
	if ctx.InsertFromConstructor().InsertValues() == nil {
		return
	}
	if ctx.InsertFromConstructor().InsertValues().ValueList() == nil {
		return
	}

	allValues := ctx.InsertFromConstructor().InsertValues().ValueList().AllValues()
	if len(allValues) > r.maxRow {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.InsertTooManyRows),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" inserts %d rows. The count exceeds %d.", r.text, len(allValues), r.maxRow),
			StartPosition: ConvertANTLRLineToPosition(r.line),
		})
	}
}

func getInsertRows(res []any) (int64, error) {
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
