package mysql

import (
	"context"
	"fmt"
	"slices"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type StatementMergeAlterTableAdvisor struct{}

func (a *StatementMergeAlterTableAdvisor) Check(
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
	mergeRule := NewStatementMergeAlterTableRule(level, string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{mergeRule})

	for _, stmt := range stmtList {
		mergeRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	// Generate advice based on collected table information
	mergeRule.generateAdvice()

	return checker.GetAdviceList(), nil
}

// tableStatement represents information about a table's statements.
type tableStatement struct {
	name     string
	count    int
	lastLine int
}

// StatementMergeAlterTableRule checks for mergeable ALTER TABLE statements.
type StatementMergeAlterTableRule struct {
	BaseRule
	text     string
	tableMap map[string]tableStatement
}

// NewStatementMergeAlterTableRule creates a new StatementMergeAlterTableRule.
func NewStatementMergeAlterTableRule(level types.Advice_Status, title string) *StatementMergeAlterTableRule {
	return &StatementMergeAlterTableRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		tableMap: make(map[string]tableStatement),
	}
}

// Name returns the rule name.
func (*StatementMergeAlterTableRule) Name() string {
	return "StatementMergeAlterTableRule"
}

// OnEnter is called when entering a parse tree node.
func (r *StatementMergeAlterTableRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*StatementMergeAlterTableRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementMergeAlterTableRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	r.tableMap[tableName] = tableStatement{
		name:     tableName,
		count:    1,
		lastLine: r.baseLine + ctx.GetStart().GetLine(),
	}
}

func (r *StatementMergeAlterTableRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableRef() == nil {
		return
	}
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	table, ok := r.tableMap[tableName]
	if !ok {
		table = tableStatement{
			name:  tableName,
			count: 0,
		}
	}
	table.count++
	table.lastLine = r.baseLine + ctx.GetStart().GetLine()
	r.tableMap[tableName] = table
}

func (r *StatementMergeAlterTableRule) generateAdvice() {
	var tableList []tableStatement
	for _, table := range r.tableMap {
		tableList = append(tableList, table)
	}
	slices.SortFunc(tableList, func(i, j tableStatement) int {
		if i.lastLine < j.lastLine {
			return -1
		}
		if i.lastLine > j.lastLine {
			return 1
		}
		return 0
	})

	for _, table := range tableList {
		if table.count > 1 {
			r.adviceList = append(r.adviceList, &types.Advice{
				Status:        r.level,
				Code:          int32(types.StatementRedundantAlterTable),
				Title:         r.title,
				Content:       fmt.Sprintf("There are %d statements to modify table `%s`", table.count, table.name),
				StartPosition: ConvertANTLRLineToPosition(table.lastLine),
			})
		}
	}
}
