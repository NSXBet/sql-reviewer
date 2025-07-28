package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// TableLimitSizeRule checks for table size limits.
type TableLimitSizeRule struct {
	BaseRule
	affectedTabNames  []string
	maxRows           int
	dbSchema          *types.DatabaseSchemaMetadata
	statementBaseLine int
}

// NewTableLimitSizeRule creates a new TableLimitSizeRule.
func NewTableLimitSizeRule(level types.Advice_Status, title string, maxRows int, dbSchema *types.DatabaseSchemaMetadata) *TableLimitSizeRule {
	return &TableLimitSizeRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		maxRows:  maxRows,
		dbSchema: dbSchema,
	}
}

// Name returns the rule name.
func (*TableLimitSizeRule) Name() string {
	return "TableLimitSizeRule"
}

// OnEnter is called when entering a parse tree node.
func (r *TableLimitSizeRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeTruncateTableStatement:
		r.checkTruncateTableStatement(ctx.(*mysql.TruncateTableStatementContext))
	case NodeTypeDropTable:
		r.checkDropTable(ctx.(*mysql.DropTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*TableLimitSizeRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableLimitSizeRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	r.statementBaseLine = ctx.GetStart().GetLine()
	r.affectedTabNames = append(r.affectedTabNames, tableName)
}

func (r *TableLimitSizeRule) checkTruncateTableStatement(ctx *mysql.TruncateTableStatementContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	r.statementBaseLine = ctx.GetStart().GetLine()
	r.affectedTabNames = append(r.affectedTabNames, tableName)
}

func (r *TableLimitSizeRule) checkDropTable(ctx *mysql.DropTableContext) {
	r.statementBaseLine = ctx.GetStart().GetLine()
	for _, tabRef := range ctx.TableRefList().AllTableRef() {
		_, tableName := mysqlparser.NormalizeMySQLTableRef(tabRef)
		r.affectedTabNames = append(r.affectedTabNames, tableName)
	}
}

func (r *TableLimitSizeRule) generateAdvice() {
	if r.dbSchema != nil && len(r.dbSchema.Schemas) != 0 {
		// Check all table size.
		for _, tabName := range r.affectedTabNames {
			tableRows := getTabRowsByName(tabName, r.dbSchema.Schemas[0].Tables)
			if tableRows >= int64(r.maxRows) {
				r.AddAdvice(&types.Advice{
					Status:        types.Advice_Status(r.level),
					Code:          int32(types.TableExceedLimitSize),
					Title:         r.title,
					Content:       fmt.Sprintf("Apply DDL on large table '%s' ( %d rows ) will lock table for a long time", tabName, tableRows),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + r.statementBaseLine),
				})
			}
		}
	}
}

func getTabRowsByName(targetTabName string, tables []*types.TableMetadata) int64 {
	for _, table := range tables {
		if table.Name == targetTabName {
			return table.RowCount
		}
	}
	return 0
}

// TableLimitSizeAdvisor is the advisor checking for table size limits.
type TableLimitSizeAdvisor struct{}

func (a *TableLimitSizeAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	var adviceList []*types.Advice

	for _, stmt := range stmtList {
		statTypeChecker := &mysqlparser.StatementTypeChecker{}
		antlr.ParseTreeWalkerDefault.Walk(statTypeChecker, stmt.Tree)

		if statTypeChecker.IsDDL {
			// Create the rule
			tableLimitSizeRule := NewTableLimitSizeRule(level, string(rule.Type), int(payload.Number), checkContext.DBSchema)

			// Create the generic checker with the rule
			checker := NewGenericChecker([]Rule{tableLimitSizeRule})

			tableLimitSizeRule.SetBaseLine(stmt.BaseLine)
			checker.SetBaseLine(stmt.BaseLine)
			antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)

			// Generate advice based on collected table information
			tableLimitSizeRule.generateAdvice()

			adviceList = append(adviceList, checker.GetAdviceList()...)
		}
	}

	return adviceList, nil
}