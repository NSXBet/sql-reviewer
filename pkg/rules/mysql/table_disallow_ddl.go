package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// TableDisallowDDLRule is the ANTLR-based implementation for checking disallow DDL on specific tables
type TableDisallowDDLRule struct {
	BaseAntlrRule
	disallowList []string
}

// NewTableDisallowDDLRule creates a new ANTLR-based table disallow DDL rule
func NewTableDisallowDDLRule(level types.SQLReviewRuleLevel, title string, disallowList []string) *TableDisallowDDLRule {
	return &TableDisallowDDLRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		disallowList: disallowList,
	}
}

// Name returns the rule name
func (*TableDisallowDDLRule) Name() string {
	return "TableDisallowDDLRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableDisallowDDLRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeDropTable:
		r.checkDropTable(ctx.(*mysql.DropTableContext))
	case NodeTypeRenameTableStatement:
		r.checkRenameTableStatement(ctx.(*mysql.RenameTableStatementContext))
	case NodeTypeTruncateTableStatement:
		r.checkTruncateTableStatement(ctx.(*mysql.TruncateTableStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableDisallowDDLRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableDisallowDDLRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	if tableName == "" {
		return
	}
	r.checkTableName(tableName, ctx.GetStart().GetLine())
}

func (r *TableDisallowDDLRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	if tableName == "" {
		return
	}
	r.checkTableName(tableName, ctx.GetStart().GetLine())
}

func (r *TableDisallowDDLRule) checkDropTable(ctx *mysql.DropTableContext) {
	for _, tableRef := range ctx.TableRefList().AllTableRef() {
		_, tableName := mysqlparser.NormalizeMySQLTableRef(tableRef)
		if tableName == "" {
			continue
		}
		r.checkTableName(tableName, ctx.GetStart().GetLine())
	}
}

func (r *TableDisallowDDLRule) checkRenameTableStatement(ctx *mysql.RenameTableStatementContext) {
	for _, renamePair := range ctx.AllRenamePair() {
		_, tableName := mysqlparser.NormalizeMySQLTableRef(renamePair.TableRef())
		if tableName == "" {
			continue
		}
		r.checkTableName(tableName, ctx.GetStart().GetLine())
	}
}

func (r *TableDisallowDDLRule) checkTruncateTableStatement(ctx *mysql.TruncateTableStatementContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	if tableName == "" {
		return
	}
	r.checkTableName(tableName, ctx.GetStart().GetLine())
}

func (r *TableDisallowDDLRule) checkTableName(tableName string, line int) {
	for _, disallow := range r.disallowList {
		if tableName == disallow {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.TableDisallowDDL),
				Title:         r.title,
				Content:       fmt.Sprintf("DDL is disallowed on table %s.", tableName),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + line),
			})
			return
		}
	}
}

// TableDisallowDDLAdvisor is the advisor using ANTLR parser for table disallow DDL checking
type TableDisallowDDLAdvisor struct{}

// Check performs the ANTLR-based table disallow DDL check
func (a *TableDisallowDDLAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	// Create the rule
	tableDisallowDDLRule := NewTableDisallowDDLRule(types.SQLReviewRuleLevel(level), string(rule.Type), payload.List)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{tableDisallowDDLRule})

	for _, stmtNode := range root {
		tableDisallowDDLRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
