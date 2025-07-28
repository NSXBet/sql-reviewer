package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/pkg/errors"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// TableRequireCollationRule is the ANTLR-based implementation for checking table require collation
type TableRequireCollationRule struct {
	BaseAntlrRule
}

// NewTableRequireCollationRule creates a new ANTLR-based table require collation rule
func NewTableRequireCollationRule(level types.SQLReviewRuleLevel, title string) *TableRequireCollationRule {
	return &TableRequireCollationRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*TableRequireCollationRule) Name() string {
	return "TableRequireCollationRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableRequireCollationRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeCreateTable {
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableRequireCollationRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableRequireCollationRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	if tableName == "" {
		return
	}

	hasCollation := false
	if ctx.CreateTableOptions() != nil {
		for _, tableOption := range ctx.CreateTableOptions().AllCreateTableOption() {
			if tableOption.DefaultCollation() != nil {
				hasCollation = true
				break
			}
		}
	}
	if !hasCollation {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.NoCollation),
			Title:         r.title,
			Content:       fmt.Sprintf("Table %s does not have a collation specified", tableName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// TableRequireCollationAdvisor is the advisor using ANTLR parser for table require collation checking
type TableRequireCollationAdvisor struct{}

// Check performs the ANTLR-based table require collation check
func (a *TableRequireCollationAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	tableRequireCollationRule := NewTableRequireCollationRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{tableRequireCollationRule})

	for _, stmtNode := range root {
		tableRequireCollationRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}