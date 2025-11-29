package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// ColumnDisallowDropInIndexRule is the ANTLR-based implementation for checking disallow DROP COLUMN in index
type ColumnDisallowDropInIndexRule struct {
	BaseAntlrRule
	tables  map[string]map[string]bool // table -> column -> isInIndex
	catalog *catalog.Finder
}

// NewColumnDisallowDropInIndexRule creates a new ANTLR-based column disallow drop in index rule
func NewColumnDisallowDropInIndexRule(
	level types.SQLReviewRuleLevel,
	title string,
	catalogFinder *catalog.Finder,
) *ColumnDisallowDropInIndexRule {
	return &ColumnDisallowDropInIndexRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		tables:  make(map[string]map[string]bool),
		catalog: catalogFinder,
	}
}

// Name returns the rule name
func (*ColumnDisallowDropInIndexRule) Name() string {
	return "ColumnDisallowDropInIndexRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnDisallowDropInIndexRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnDisallowDropInIndexRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnDisallowDropInIndexRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil || ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil || tableElement.TableConstraintDef() == nil {
			continue
		}
		if tableElement.TableConstraintDef().GetType_() == nil {
			continue
		}

		switch tableElement.TableConstraintDef().GetType_().GetTokenType() {
		case mysql.MySQLParserINDEX_SYMBOL,
			mysql.MySQLParserKEY_SYMBOL,
			mysql.MySQLParserPRIMARY_SYMBOL,
			mysql.MySQLParserUNIQUE_SYMBOL:
			if tableElement.TableConstraintDef().KeyListVariants() == nil {
				continue
			}
			columnList := mysqlparser.NormalizeKeyListVariants(tableElement.TableConstraintDef().KeyListVariants())
			for _, column := range columnList {
				r.markColumnAsIndexed(tableName, column)
			}
		case mysql.MySQLParserFOREIGN_SYMBOL:
			if tableElement.TableConstraintDef().KeyList() == nil {
				continue
			}
			columnList := mysqlparser.NormalizeKeyList(tableElement.TableConstraintDef().KeyList())
			for _, column := range columnList {
				r.markColumnAsIndexed(tableName, column)
			}
		}
	}
}

func (r *ColumnDisallowDropInIndexRule) markColumnAsIndexed(tableName, columnName string) {
	if r.tables[tableName] == nil {
		r.tables[tableName] = make(map[string]bool)
	}
	r.tables[tableName][columnName] = true
}

func (r *ColumnDisallowDropInIndexRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

	// Check existing indexes from catalog if available
	if r.catalog != nil {
		// Get existing indexes for this table from catalog
		r.loadExistingIndexes(tableName)
	}

	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil || item.DROP_SYMBOL() == nil || item.ColumnInternalRef() == nil {
			continue
		}

		columnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
		if !r.canDrop(tableName, columnName) {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.DropIndexColumn),
				Title:         r.title,
				Content:       fmt.Sprintf("`%s`.`%s` cannot drop index column", tableName, columnName),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + item.GetStart().GetLine()),
			})
		}
	}
}

func (r *ColumnDisallowDropInIndexRule) loadExistingIndexes(tableName string) {
	if r.catalog == nil {
		return
	}

	// Try to find existing indexes in the catalog
	table := r.catalog.Origin.FindTable(&catalog.TableFind{
		SchemaName: "", // MySQL doesn't have schema
		TableName:  tableName,
	})

	if table != nil {
		indexMap := table.Index(&catalog.TableIndexFind{
			SchemaName: "",
			TableName:  tableName,
		})
		if indexMap != nil {
			for _, index := range *indexMap {
				for _, expr := range index.ExpressionList() {
					r.markColumnAsIndexed(tableName, expr)
				}
			}
		}
	}
}

func (r *ColumnDisallowDropInIndexRule) canDrop(tableName, columnName string) bool {
	if tableMap, exists := r.tables[tableName]; exists {
		if _, inIndex := tableMap[columnName]; inIndex {
			return false
		}
	}
	return true
}

// ColumnDisallowDropInIndexAdvisor is the advisor using ANTLR parser for column disallow drop in index checking
type ColumnDisallowDropInIndexAdvisor struct{}

// Check performs the ANTLR-based column disallow drop in index check
func (a *ColumnDisallowDropInIndexAdvisor) Check(
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

	// Get catalog finder (nil is acceptable for this rule)
	catalogFinder := getCatalogFinder(checkContext)

	// Create the rule with catalog support
	dropInIndexRule := NewColumnDisallowDropInIndexRule(types.SQLReviewRuleLevel(level), string(rule.Type), catalogFinder)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{dropInIndexRule})

	for _, stmtNode := range root {
		dropInIndexRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
