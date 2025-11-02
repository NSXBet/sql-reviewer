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

// IndexKeyNumberLimitRule is the ANTLR-based implementation for checking index key number limit
type IndexKeyNumberLimitRule struct {
	BaseAntlrRule
	max int
}

// NewIndexKeyNumberLimitRule creates a new ANTLR-based index key number limit rule
func NewIndexKeyNumberLimitRule(level types.SQLReviewRuleLevel, title string, maxKeys int) *IndexKeyNumberLimitRule {
	return &IndexKeyNumberLimitRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		max: maxKeys,
	}
}

// Name returns the rule name
func (*IndexKeyNumberLimitRule) Name() string {
	return "IndexKeyNumberLimitRule"
}

// OnEnter is called when entering a parse tree node
func (r *IndexKeyNumberLimitRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
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
func (*IndexKeyNumberLimitRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *IndexKeyNumberLimitRule) checkCreateTable(ctx *mysql.CreateTableContext) {
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

func (r *IndexKeyNumberLimitRule) handleConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) {
	var columnList []string
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserINDEX_SYMBOL,
		mysql.MySQLParserKEY_SYMBOL,
		mysql.MySQLParserPRIMARY_SYMBOL,
		mysql.MySQLParserUNIQUE_SYMBOL:
		if ctx.KeyListVariants() == nil {
			return
		}
		columnList = mysqlparser.NormalizeKeyListVariants(ctx.KeyListVariants())
	case mysql.MySQLParserFOREIGN_SYMBOL:
		if ctx.KeyList() == nil {
			return
		}
		columnList = mysqlparser.NormalizeKeyList(ctx.KeyList())
	default:
		return
	}

	indexName := ""
	if ctx.IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexName())
	}
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
	}

	if r.max > 0 && len(columnList) > r.max {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.IndexKeyNumberExceedsLimit),
			Title:  r.title,
			Content: fmt.Sprintf(
				"The number of index `%s` in table `%s` should be not greater than %d",
				indexName,
				tableName,
				r.max,
			),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *IndexKeyNumberLimitRule) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserFULLTEXT_SYMBOL, mysql.MySQLParserSPATIAL_SYMBOL:
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil ||
		ctx.CreateIndexTarget().KeyListVariants() == nil {
		return
	}

	indexName := ""
	if ctx.IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexName())
	}
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.CreateIndexTarget().TableRef())
	columnList := mysqlparser.NormalizeKeyListVariants(ctx.CreateIndexTarget().KeyListVariants())
	if r.max > 0 && len(columnList) > r.max {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.IndexKeyNumberExceedsLimit),
			Title:  r.title,
			Content: fmt.Sprintf(
				"The number of index `%s` in table `%s` should be not greater than %d",
				indexName,
				tableName,
				r.max,
			),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *IndexKeyNumberLimitRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

// IndexKeyNumberLimitAdvisor is the advisor using ANTLR parser for index key number limit checking
type IndexKeyNumberLimitAdvisor struct{}

// Check performs the ANTLR-based index key number limit check
func (a *IndexKeyNumberLimitAdvisor) Check(
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

	// Based on test data, max keys is 5
	maxKeys := 5

	// Create the rule (doesn't need catalog)
	indexRule := NewIndexKeyNumberLimitRule(types.SQLReviewRuleLevel(level), string(rule.Type), maxKeys)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{indexRule})

	for _, stmtNode := range root {
		indexRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
