package mysql

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// IndexTotalNumberLimitRule is the ANTLR-based implementation for checking index total number limit
type IndexTotalNumberLimitRule struct {
	BaseAntlrRule
	max          int
	lineForTable map[string]int
	catalog      *catalog.Finder
}

// NewIndexTotalNumberLimitRule creates a new ANTLR-based index total number limit rule
func NewIndexTotalNumberLimitRule(
	level types.SQLReviewRuleLevel,
	title string,
	maxIndexes int,
	catalog *catalog.Finder,
) *IndexTotalNumberLimitRule {
	return &IndexTotalNumberLimitRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		max:          maxIndexes,
		lineForTable: make(map[string]int),
		catalog:      catalog,
	}
}

// Name returns the rule name
func (*IndexTotalNumberLimitRule) Name() string {
	return "IndexTotalNumberLimitRule"
}

// OnEnter is called when entering a parse tree node
func (r *IndexTotalNumberLimitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeCreateIndex:
		r.checkCreateIndex(ctx.(*mysql.CreateIndexContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*IndexTotalNumberLimitRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

// GetAdviceList returns the accumulated advice, implementing post-processing
func (r *IndexTotalNumberLimitRule) GetAdviceList() []*types.Advice {
	return r.generateAdvice()
}

func (r *IndexTotalNumberLimitRule) generateAdvice() []*types.Advice {
	type tableName struct {
		name string
		line int
	}
	var tableList []tableName

	for k, v := range r.lineForTable {
		tableList = append(tableList, tableName{
			name: k,
			line: v,
		})
	}
	slices.SortFunc(tableList, func(i, j tableName) int {
		if i.line < j.line {
			return -1
		}
		if i.line > j.line {
			return 1
		}
		return 0
	})

	for _, table := range tableList {
		if r.catalog != nil {
			tableInfo := r.catalog.Final.FindTable(&catalog.TableFind{TableName: table.name})
			if tableInfo != nil && tableInfo.CountIndex() > r.max {
				r.AddAdvice(&types.Advice{
					Status: types.Advice_Status(r.level),
					Code:   int32(types.IndexCountExceedsLimit),
					Title:  r.title,
					Content: fmt.Sprintf(
						"The count of index in table `%s` should be no more than %d, but found %d",
						table.name,
						r.max,
						tableInfo.CountIndex(),
					),
					StartPosition: ConvertANTLRLineToPosition(table.line),
				})
			}
		}
	}

	return r.adviceList
}

func (r *IndexTotalNumberLimitRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	r.lineForTable[tableName] = r.baseLine + ctx.GetStart().GetLine()
}

func (r *IndexTotalNumberLimitRule) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil {
		return
	}
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.CreateIndexTarget().TableRef())
	r.lineForTable[tableName] = r.baseLine + ctx.GetStart().GetLine()
}

func (r *IndexTotalNumberLimitRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
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
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		switch {
		// add column.
		case item.ADD_SYMBOL() != nil:
			switch {
			// add single columns.
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				r.checkFieldDefinitionContext(tableName, item.FieldDefinition())
			// add multi columns.
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					r.checkFieldDefinitionContext(tableName, tableElement.ColumnDefinition().FieldDefinition())
				}
				// add constraint.
			case item.TableConstraintDef() != nil:
				r.checkTableConstraintDef(tableName, item.TableConstraintDef())
			}
		// change column.
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil:
			r.checkFieldDefinitionContext(tableName, item.FieldDefinition())
		// modify column.
		case item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.FieldDefinition() != nil:
			r.checkFieldDefinitionContext(tableName, item.FieldDefinition())
		default:
			continue
		}
	}
}

func (r *IndexTotalNumberLimitRule) checkFieldDefinitionContext(tableName string, ctx mysql.IFieldDefinitionContext) {
	for _, attr := range ctx.AllColumnAttribute() {
		if attr == nil || attr.GetValue() == nil {
			continue
		}
		switch attr.GetValue().GetTokenType() {
		case mysql.MySQLParserPRIMARY_SYMBOL, mysql.MySQLParserUNIQUE_SYMBOL:
			r.lineForTable[tableName] = r.baseLine + ctx.GetStart().GetLine()
		}
	}
}

func (r *IndexTotalNumberLimitRule) checkTableConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
	if ctx.GetType_() == nil {
		return
	}
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserPRIMARY_SYMBOL,
		mysql.MySQLParserUNIQUE_SYMBOL,
		mysql.MySQLParserKEY_SYMBOL,
		mysql.MySQLParserINDEX_SYMBOL,
		mysql.MySQLParserFULLTEXT_SYMBOL:
		r.lineForTable[tableName] = r.baseLine + ctx.GetStart().GetLine()
	}
}

// IndexTotalNumberLimitAdvisor is the advisor using ANTLR parser for index total number limit checking
type IndexTotalNumberLimitAdvisor struct{}

// Check performs the ANTLR-based index total number limit check using payload
func (a *IndexTotalNumberLimitAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.Context,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Parse the maximum index count from rule payload
	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	maxIndexes := int(payload.Number)

	// Get catalog finder - skip validation if not available
	catalogFinder := getCatalogFinder(checkContext)
	if catalogFinder == nil {
		return nil, nil
	}

	// Update the catalog with the current statements to simulate the final state
	// Convert our parse results to the format expected by catalog walkthrough
	catalogAST := catalog.ConvertMySQLParseResults(root)

	// Walk through the statements to update the catalog state
	if err := catalogFinder.WalkThrough(catalogAST); err != nil {
		// If walkthrough fails, continue without updated catalog but log the error
		// This ensures the rule still works even if catalog update fails
		slog.Warn("catalog walkthrough failed", "error", err, "rule", rule.Type)
	}

	// Create the rule with updated catalog
	indexTotalRule := NewIndexTotalNumberLimitRule(types.SQLReviewRuleLevel(level), string(rule.Type), maxIndexes, catalogFinder)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{indexTotalRule})

	for _, stmtNode := range root {
		indexTotalRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
