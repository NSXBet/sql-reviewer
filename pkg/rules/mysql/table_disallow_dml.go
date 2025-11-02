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

// TableDisallowDMLRule is the ANTLR-based implementation for checking disallow DML on specific tables
type TableDisallowDMLRule struct {
	BaseAntlrRule
	disallowList []string
}

// NewTableDisallowDMLRule creates a new ANTLR-based table disallow DML rule
func NewTableDisallowDMLRule(level types.SQLReviewRuleLevel, title string, disallowList []string) *TableDisallowDMLRule {
	return &TableDisallowDMLRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		disallowList: disallowList,
	}
}

// Name returns the rule name
func (*TableDisallowDMLRule) Name() string {
	return "TableDisallowDMLRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableDisallowDMLRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeDeleteStatement:
		r.checkDeleteStatement(ctx.(*mysql.DeleteStatementContext))
	case NodeTypeInsertStatement:
		r.checkInsertStatement(ctx.(*mysql.InsertStatementContext))
	case NodeTypeSelectStatementWithInto:
		r.checkSelectStatementWithInto(ctx.(*mysql.SelectStatementWithIntoContext))
	case NodeTypeUpdateStatement:
		r.checkUpdateStatement(ctx.(*mysql.UpdateStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableDisallowDMLRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableDisallowDMLRule) checkDeleteStatement(ctx *mysql.DeleteStatementContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	if tableName == "" {
		return
	}
	r.checkTableName(tableName, ctx.GetStart().GetLine())
}

func (r *TableDisallowDMLRule) checkInsertStatement(ctx *mysql.InsertStatementContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	if tableName == "" {
		return
	}
	r.checkTableName(tableName, ctx.GetStart().GetLine())
}

func (r *TableDisallowDMLRule) checkSelectStatementWithInto(ctx *mysql.SelectStatementWithIntoContext) {
	// Only check text string literal for now.
	if ctx.IntoClause() == nil || ctx.IntoClause().TextStringLiteral() == nil {
		return
	}
	tableName := ctx.IntoClause().TextStringLiteral().GetText()
	// Remove quotes if present
	if len(tableName) > 2 && ((tableName[0] == '"' && tableName[len(tableName)-1] == '"') ||
		(tableName[0] == '\'' && tableName[len(tableName)-1] == '\'')) {
		tableName = tableName[1 : len(tableName)-1]
	}
	r.checkTableName(tableName, ctx.GetStart().GetLine())
}

func (r *TableDisallowDMLRule) checkUpdateStatement(ctx *mysql.UpdateStatementContext) {
	if ctx.TableReferenceList() == nil {
		return
	}
	// Simple approach: check the first table reference
	for _, tableRef := range ctx.TableReferenceList().AllTableReference() {
		if tableRef.TableFactor() != nil && tableRef.TableFactor().SingleTable() != nil {
			_, tableName := mysqlparser.NormalizeMySQLTableRef(tableRef.TableFactor().SingleTable().TableRef())
			if tableName != "" {
				r.checkTableName(tableName, ctx.GetStart().GetLine())
			}
		}
	}
}

func (r *TableDisallowDMLRule) checkTableName(tableName string, line int) {
	for _, disallow := range r.disallowList {
		if tableName == disallow {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.TableDisallowDML),
				Title:         r.title,
				Content:       fmt.Sprintf("DML is disallowed on table %s.", tableName),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + line),
			})
			return
		}
	}
}

// TableDisallowDMLAdvisor is the advisor using ANTLR parser for table disallow DML checking
type TableDisallowDMLAdvisor struct{}

// Check performs the ANTLR-based table disallow DML check
func (a *TableDisallowDMLAdvisor) Check(
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
	tableDisallowDMLRule := NewTableDisallowDMLRule(types.SQLReviewRuleLevel(level), string(rule.Type), payload.List)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{tableDisallowDMLRule})

	for _, stmtNode := range root {
		tableDisallowDMLRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
