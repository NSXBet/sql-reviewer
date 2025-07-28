package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type TableNoDuplicateIndexAdvisor struct{}

func (a *TableNoDuplicateIndexAdvisor) Check(
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
	noDuplicateIndexRule := NewTableNoDuplicateIndexRule(level, string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{noDuplicateIndexRule})

	for _, stmt := range stmtList {
		noDuplicateIndexRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

type duplicateIndex struct {
	indexName string
	// BTREE, HASH, etc.
	indexType string
	unique    bool
	fulltext  bool
	spatial   bool
	table     string
	columns   []string
	// line is the line number of the index definition.
	line int
}

// TableNoDuplicateIndexRule checks for no duplicate index in table.
type TableNoDuplicateIndexRule struct {
	BaseRule
	indexList []duplicateIndex
}

// NewTableNoDuplicateIndexRule creates a new TableNoDuplicateIndexRule.
func NewTableNoDuplicateIndexRule(level types.Advice_Status, title string) *TableNoDuplicateIndexRule {
	return &TableNoDuplicateIndexRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		indexList: []duplicateIndex{},
	}
}

// Name returns the rule name.
func (*TableNoDuplicateIndexRule) Name() string {
	return "TableNoDuplicateIndexRule"
}

// OnEnter is called when entering a parse tree node.
func (r *TableNoDuplicateIndexRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeCreateIndex:
		r.checkCreateIndex(ctx.(*mysql.CreateIndexContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (*TableNoDuplicateIndexRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableNoDuplicateIndexRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	// Build suspect index list.
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil || tableElement.TableConstraintDef() == nil {
			continue
		}
		r.handleConstraintDef(tableName, tableElement.TableConstraintDef())
	}
	// Check for duplicate index.
	if index := hasDuplicateIndexes(r.indexList); index != nil {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.DuplicateIndexInTable),
			Title:         r.title,
			Content:       fmt.Sprintf("`%s` has duplicate index `%s`", tableName, index.indexName),
			StartPosition: ConvertANTLRLineToPosition(index.line),
		})
	}
}

func (r *TableNoDuplicateIndexRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableRef() == nil {
		return
	}
	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil ||
		ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if alterListItem == nil {
			continue
		}
		if alterListItem.ADD_SYMBOL() != nil && alterListItem.TableConstraintDef() != nil {
			r.handleConstraintDef(tableName, alterListItem.TableConstraintDef())
		}
	}
	if index := hasDuplicateIndexes(r.indexList); index != nil {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.DuplicateIndexInTable),
			Title:         r.title,
			Content:       fmt.Sprintf("`%s` has duplicate index `%s`", tableName, index.indexName),
			StartPosition: ConvertANTLRLineToPosition(index.line),
		})
	}
}

func (r *TableNoDuplicateIndexRule) handleConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserINDEX_SYMBOL,
		mysql.MySQLParserKEY_SYMBOL,
		mysql.MySQLParserPRIMARY_SYMBOL,
		mysql.MySQLParserUNIQUE_SYMBOL,
		mysql.MySQLParserFULLTEXT_SYMBOL,
		mysql.MySQLParserSPATIAL_SYMBOL:
	default:
		return
	}

	index := duplicateIndex{
		indexType: "BTREE",
		line:      r.baseLine + ctx.GetStart().GetLine(),
		table:     tableName,
	}
	if ctx.KeyListVariants() != nil {
		index.columns = mysqlparser.NormalizeKeyListVariants(ctx.KeyListVariants())
	}
	if ctx.UNIQUE_SYMBOL() != nil {
		index.unique = true
	} else if ctx.FULLTEXT_SYMBOL() != nil {
		index.fulltext = true
	} else if ctx.SPATIAL_SYMBOL() != nil {
		index.spatial = true
	}
	if ctx.IndexNameAndType() != nil {
		if ctx.IndexNameAndType().IndexName() != nil {
			index.indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
		}
		if ctx.IndexNameAndType().IndexType() != nil {
			index.indexType = ctx.IndexNameAndType().IndexType().GetText()
		}
	}
	r.indexList = append(r.indexList, index)
}

func (r *TableNoDuplicateIndexRule) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil ||
		ctx.CreateIndexTarget().KeyListVariants() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.CreateIndexTarget().TableRef())
	index := duplicateIndex{
		indexType: "BTREE",
		line:      r.baseLine + ctx.GetStart().GetLine(),
		table:     tableName,
	}
	if ctx.UNIQUE_SYMBOL() != nil {
		index.unique = true
	} else if ctx.FULLTEXT_SYMBOL() != nil {
		index.fulltext = true
	} else if ctx.SPATIAL_SYMBOL() != nil {
		index.spatial = true
	}
	if ctx.IndexName() != nil {
		index.indexName = mysqlparser.NormalizeIndexName(ctx.IndexName())
	}
	if ctx.IndexNameAndType() != nil {
		if ctx.IndexNameAndType().IndexName() != nil {
			index.indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
		}
		if ctx.IndexNameAndType().IndexType() != nil {
			index.indexType = ctx.IndexNameAndType().IndexType().GetText()
		}
	}
	if ctx.IndexTypeClause() != nil && ctx.IndexTypeClause().IndexType() != nil {
		index.indexType = ctx.IndexTypeClause().IndexType().GetText()
	}

	index.columns = mysqlparser.NormalizeKeyListVariants(ctx.CreateIndexTarget().KeyListVariants())
	r.indexList = append(r.indexList, index)
	if index := hasDuplicateIndexes(r.indexList); index != nil {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          int32(types.DuplicateIndexInTable),
			Title:         r.title,
			Content:       fmt.Sprintf("`%s` has duplicate index `%s`", tableName, index.indexName),
			StartPosition: ConvertANTLRLineToPosition(index.line),
		})
	}
}

// hasDuplicateIndexes returns the first duplicate index if found, otherwise nil.
func hasDuplicateIndexes(indexList []duplicateIndex) *duplicateIndex {
	seen := make(map[string]struct{})
	for _, index := range indexList {
		key := indexKey(index)
		if _, exists := seen[key]; exists {
			return &index
		}
		seen[key] = struct{}{}
	}
	return nil
}

// indexKey returns a string key for the index with the index type and columns.
func indexKey(index duplicateIndex) string {
	parts := []string{}
	if index.unique {
		parts = append(parts, "unique")
	}
	if index.fulltext {
		parts = append(parts, "fulltext")
	}
	if index.spatial {
		parts = append(parts, "spatial")
	}
	parts = append(parts, index.indexType)
	parts = append(parts, index.table)
	parts = append(parts, index.columns...)
	return strings.Join(parts, "-")
}
