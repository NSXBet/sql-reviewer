package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// ColumnDisallowChangeTypeRule is the ANTLR-based implementation for checking disallow changing column type
type ColumnDisallowChangeTypeRule struct {
	BaseAntlrRule
	text    string
	catalog *catalog.Finder
}

// NewColumnDisallowChangeTypeRule creates a new ANTLR-based column disallow change type rule
func NewColumnDisallowChangeTypeRule(
	level types.SQLReviewRuleLevel,
	title string,
	catalog *catalog.Finder,
) *ColumnDisallowChangeTypeRule {
	return &ColumnDisallowChangeTypeRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		catalog: catalog,
	}
}

// Name returns the rule name
func (*ColumnDisallowChangeTypeRule) Name() string {
	return "ColumnDisallowChangeTypeRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnDisallowChangeTypeRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnDisallowChangeTypeRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnDisallowChangeTypeRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	// alter table add column, change column, modify column.
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		var columnName string
		switch {
		// change column
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil:
			// only focus on old column-name.
			columnName = mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
		// MODIFY COLUMN
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			columnName = mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
		default:
			continue
		}
		if item.FieldDefinition() != nil && item.FieldDefinition().DataType() != nil {
			r.changeColumnType(tableName, columnName, item.FieldDefinition().DataType())
		}
	}
}

func normalizeColumnType(tp string) string {
	switch strings.ToLower(tp) {
	case "tinyint":
		return "tinyint(4)"
	case "tinyint unsigned":
		return "tinyint(4) unsigned"
	case "smallint":
		return "smallint(6)"
	case "smallint unsigned":
		return "smallint(6) unsigned"
	case "mediumint":
		return "mediumint(9)"
	case "mediumint unsigned":
		return "mediumint(9) unsigned"
	case "int":
		return "int(11)"
	case "int unsigned":
		return "int(11) unsigned"
	case "bigint":
		return "bigint(20)"
	case "bigint unsigned":
		return "bigint(20) unsigned"
	default:
		return strings.ToLower(tp)
	}
}

func (r *ColumnDisallowChangeTypeRule) changeColumnType(tableName, columnName string, dataType mysql.IDataTypeContext) {
	tp := dataType.GetParser().GetTokenStream().GetTextFromRuleContext(dataType)
	column := r.catalog.Origin.FindColumn(&catalog.ColumnFind{
		SchemaName: "", // MySQL default schema
		TableName:  tableName,
		ColumnName: columnName,
	})

	if column == nil {
		return
	}

	if normalizeColumnType(column.Type()) != normalizeColumnType(tp) {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.ChangeColumnType),
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" changes column type", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + dataType.GetStart().GetLine()),
		})
	}
}

// ColumnDisallowChangeTypeAdvisor is the advisor using ANTLR parser for column disallow change type checking
type ColumnDisallowChangeTypeAdvisor struct{}

// CheckContext represents the context needed for column disallow change type checking
type CheckContext struct {
	Catalog *catalog.Finder
}

// Check performs the ANTLR-based column disallow change type check
func (a *ColumnDisallowChangeTypeAdvisor) Check(
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

	// Get catalog from context
	var catalogFinder *catalog.Finder
	if checkContext.Catalog != nil {
		catalogFinder = checkContext.Catalog.GetFinder()
	} else if checkContext.DBSchema != nil {
		// Create catalog from database schema if available
		finderCtx := &catalog.FinderContext{
			CheckIntegrity:      true,
			EngineType:          checkContext.DBType,
			IgnoreCaseSensitive: !checkContext.IsObjectCaseSensitive,
		}
		catalogFinder = catalog.NewFinder(checkContext.DBSchema, finderCtx)
	} else {
		return nil, fmt.Errorf("no catalog or database schema provided in context")
	}

	// Create the rule with the catalog from context
	changeTypeRule := NewColumnDisallowChangeTypeRule(types.SQLReviewRuleLevel(level), string(rule.Type), catalogFinder)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{changeTypeRule})

	for _, stmtNode := range root {
		changeTypeRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
