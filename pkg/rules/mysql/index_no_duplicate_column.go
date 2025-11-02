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

// IndexNoDuplicateColumnRule is the ANTLR-based implementation for checking no duplicate columns in index
type IndexNoDuplicateColumnRule struct {
	BaseAntlrRule
}

// NewIndexNoDuplicateColumnRule creates a new ANTLR-based index no duplicate column rule
func NewIndexNoDuplicateColumnRule(level types.SQLReviewRuleLevel, title string) *IndexNoDuplicateColumnRule {
	return &IndexNoDuplicateColumnRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*IndexNoDuplicateColumnRule) Name() string {
	return "IndexNoDuplicateColumnRule"
}

// OnEnter is called when entering a parse tree node
func (r *IndexNoDuplicateColumnRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
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

// OnExit is called when exiting a parse tree node
func (*IndexNoDuplicateColumnRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *IndexNoDuplicateColumnRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		if tableElement.TableConstraintDef() == nil {
			continue
		}
		r.handleConstraintDef(tableName, tableElement.TableConstraintDef())
	}
}

func (r *IndexNoDuplicateColumnRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}
	if ctx.TableRef() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if alterListItem == nil {
			continue
		}

		switch {
		// add index.
		case alterListItem.ADD_SYMBOL() != nil && alterListItem.TableConstraintDef() != nil:
			r.handleConstraintDef(tableName, alterListItem.TableConstraintDef())
		default:
			continue
		}
	}
}

func (r *IndexNoDuplicateColumnRule) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserFULLTEXT_SYMBOL, mysql.MySQLParserSPATIAL_SYMBOL:
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil ||
		ctx.CreateIndexTarget().KeyListVariants() == nil {
		return
	}
	indexType := ctx.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(
		ctx.GetStart().GetTokenIndex(),
		ctx.CreateIndexTarget().KeyListVariants().GetStart().GetTokenIndex()-1,
	))

	indexName := ""
	if ctx.IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexName())
		indexType = ctx.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(
			ctx.GetStart().GetTokenIndex(),
			ctx.IndexName().GetStart().GetTokenIndex()-1,
		))
	}
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
		indexType = ctx.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(
			ctx.GetStart().GetTokenIndex(),
			ctx.IndexNameAndType().GetStart().GetTokenIndex()-1,
		))
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.CreateIndexTarget().TableRef())
	columnList := mysqlparser.NormalizeKeyListVariants(ctx.CreateIndexTarget().KeyListVariants())
	if column, duplicate := r.hasDuplicateColumn(columnList); duplicate {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.IndexDuplicateColumn),
			Title:         r.title,
			Content:       fmt.Sprintf("%s`%s` has duplicate column `%s`.`%s`", indexType, indexName, tableName, column),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *IndexNoDuplicateColumnRule) handleConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
	var columnList []string
	indexType := ""
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserINDEX_SYMBOL,
		mysql.MySQLParserKEY_SYMBOL,
		mysql.MySQLParserPRIMARY_SYMBOL,
		mysql.MySQLParserUNIQUE_SYMBOL:
		if ctx.KeyListVariants() == nil {
			return
		}
		columnList = mysqlparser.NormalizeKeyListVariants(ctx.KeyListVariants())
		indexType = ctx.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(
			ctx.GetStart().GetTokenIndex(),
			ctx.KeyListVariants().GetStart().GetTokenIndex()-1,
		))
	case mysql.MySQLParserFOREIGN_SYMBOL:
		if ctx.KeyList() == nil {
			return
		}
		columnList = mysqlparser.NormalizeKeyList(ctx.KeyList())
		indexType = ctx.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(
			ctx.GetStart().GetTokenIndex(),
			ctx.KeyList().GetStart().GetTokenIndex()-1,
		))
	default:
		return
	}

	indexName := ""
	if ctx.IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexName())
		indexType = ctx.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(
			ctx.GetStart().GetTokenIndex(),
			ctx.IndexName().GetStart().GetTokenIndex()-1,
		))
	}
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
		indexType = ctx.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(
			ctx.GetStart().GetTokenIndex(),
			ctx.IndexNameAndType().GetStart().GetTokenIndex()-1,
		))
	}
	if column, duplicate := r.hasDuplicateColumn(columnList); duplicate {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.IndexDuplicateColumn),
			Title:         r.title,
			Content:       fmt.Sprintf("%s`%s` has duplicate column `%s`.`%s`", indexType, indexName, tableName, column),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (*IndexNoDuplicateColumnRule) hasDuplicateColumn(keyList []string) (string, bool) {
	listMap := make(map[string]struct{})
	for _, keyName := range keyList {
		if _, exists := listMap[keyName]; exists {
			return keyName, true
		}
		listMap[keyName] = struct{}{}
	}

	return "", false
}

// IndexNoDuplicateColumnAdvisor is the advisor using ANTLR parser for index no duplicate column checking
type IndexNoDuplicateColumnAdvisor struct{}

// Check performs the ANTLR-based index no duplicate column check
func (a *IndexNoDuplicateColumnAdvisor) Check(
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
	indexRule := NewIndexNoDuplicateColumnRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{indexRule})

	for _, stmtNode := range root {
		indexRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
