package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// TableNoForeignKeyRule is the ANTLR-based implementation for checking table disallow foreign key
type TableNoForeignKeyRule struct {
	BaseAntlrRule
}

// NewTableNoForeignKeyRule creates a new ANTLR-based table no foreign key rule
func NewTableNoForeignKeyRule(level types.SQLReviewRuleLevel, title string) *TableNoForeignKeyRule {
	return &TableNoForeignKeyRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*TableNoForeignKeyRule) Name() string {
	return "TableNoForeignKeyRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableNoForeignKeyRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableNoForeignKeyRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableNoForeignKeyRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil || ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement.TableConstraintDef() == nil {
			continue
		}
		r.handleTableConstraintDef(tableName, tableElement.TableConstraintDef())
	}
}

func (r *TableNoForeignKeyRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.TableRef() == nil {
		return
	}
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())

	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}
	for _, option := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		switch {
		// ADD CONSTRAINT
		case option.ADD_SYMBOL() != nil && option.TableConstraintDef() != nil:
			r.handleTableConstraintDef(tableName, option.TableConstraintDef())
		default:
			continue
		}
	}
}

func (r *TableNoForeignKeyRule) handleTableConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
	if ctx.GetType_() != nil && ctx.GetType_().GetTokenType() == mysql.MySQLParserFOREIGN_SYMBOL {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.TableHasFK),
			Title:         r.title,
			Content:       fmt.Sprintf("Foreign key is not allowed in the table `%s`", tableName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// TableNoForeignKeyAdvisor is the advisor using ANTLR parser for table no foreign key checking
type TableNoForeignKeyAdvisor struct{}

// Check performs the ANTLR-based table no foreign key check
func (a *TableNoForeignKeyAdvisor) Check(
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

	// Create the rule (doesn't need catalog)
	fkRule := NewTableNoForeignKeyRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{fkRule})

	for _, stmtNode := range root {
		fkRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
