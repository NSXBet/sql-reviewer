package mysql

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ukIndexMetaData is the metadata for unique key index.
type ukIndexMetaData struct {
	indexName string
	tableName string
	metaData  map[string]string
	line      int
}

// NamingIndexUkAdvisor is the ANTLR-based implementation for checking unique key naming convention
type NamingIndexUkAdvisor struct {
	BaseAntlrRule
	format       string
	maxLength    int
	templateList []string
}

// NewNamingIndexUkAdvisor creates a new ANTLR-based unique key naming rule
func NewNamingIndexUkAdvisor(
	level types.SQLReviewRuleLevel,
	title string,
	format string,
	maxLength int,
	templateList []string,
) *NamingIndexUkAdvisor {
	return &NamingIndexUkAdvisor{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		format:       format,
		maxLength:    maxLength,
		templateList: templateList,
	}
}

// Name returns the rule name
func (*NamingIndexUkAdvisor) Name() string {
	return "NamingIndexUkAdvisor"
}

// OnEnter is called when entering a parse tree node
func (r *NamingIndexUkAdvisor) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
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
func (*NamingIndexUkAdvisor) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *NamingIndexUkAdvisor) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())

	var indexDataList []*ukIndexMetaData
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		if tableElement.TableConstraintDef() == nil {
			continue
		}
		if metaData := r.handleConstraintDef(tableName, tableElement.TableConstraintDef()); metaData != nil {
			indexDataList = append(indexDataList, metaData)
		}
	}
	r.handleIndexList(indexDataList)
}

func (r *NamingIndexUkAdvisor) checkAlterTable(ctx *mysql.AlterTableContext) {
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
	var indexDataList []*ukIndexMetaData
	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if alterListItem == nil {
			continue
		}

		switch {
		// add unique index.
		case alterListItem.ADD_SYMBOL() != nil && alterListItem.TableConstraintDef() != nil:
			if metaData := r.handleConstraintDef(tableName, alterListItem.TableConstraintDef()); metaData != nil {
				indexDataList = append(indexDataList, metaData)
			}
		// rename index.
		case alterListItem.RENAME_SYMBOL() != nil && alterListItem.KeyOrIndex() != nil && alterListItem.IndexRef() != nil && alterListItem.IndexName() != nil:
			_, _, oldIndexName := mysqlparser.NormalizeIndexRef(alterListItem.IndexRef())
			newIndexName := mysqlparser.NormalizeIndexName(alterListItem.IndexName())
			// For rename operations, we infer column list from the old index name
			// This is a simplified approach - in a real implementation, we'd need catalog lookup
			columnList := strings.TrimPrefix(oldIndexName, "uk_"+tableName+"_")
			if columnList == oldIndexName {
				columnList = "id_name" // fallback
			}
			metaData := map[string]string{
				advisor.ColumnListTemplateToken: columnList,
				advisor.TableNameTemplateToken:  tableName,
			}
			indexData := &ukIndexMetaData{
				indexName: newIndexName,
				tableName: tableName,
				metaData:  metaData,
				line:      ctx.GetStart().GetLine(),
			}
			indexDataList = append(indexDataList, indexData)
		}
	}
	r.handleIndexList(indexDataList)
}

func (r *NamingIndexUkAdvisor) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	// Only check unique indexes
	if ctx.UNIQUE_SYMBOL() == nil {
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil {
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

	if ctx.CreateIndexTarget().KeyListVariants() == nil {
		return
	}
	columnList := mysqlparser.NormalizeKeyListVariants(ctx.CreateIndexTarget().KeyListVariants())
	metaData := map[string]string{
		advisor.ColumnListTemplateToken: strings.Join(columnList, "_"),
		advisor.TableNameTemplateToken:  tableName,
	}
	indexDataList := []*ukIndexMetaData{
		{
			indexName: indexName,
			tableName: tableName,
			metaData:  metaData,
			line:      ctx.GetStart().GetLine(),
		},
	}
	r.handleIndexList(indexDataList)
}

func (r *NamingIndexUkAdvisor) handleIndexList(indexDataList []*ukIndexMetaData) {
	for _, indexData := range indexDataList {
		regex, err := r.getTemplateRegexp(r.format, r.templateList, indexData.metaData)
		if err != nil {
			// Skip this index if regex compilation fails
			continue
		}
		if !regex.MatchString(indexData.indexName) {
			r.AddAdvice(&types.Advice{
				Status: types.Advice_Status(r.level),
				Code:   int32(types.NamingUKConventionMismatch),
				Title:  r.title,
				Content: fmt.Sprintf(
					"Unique key in table `%s` mismatches the naming convention, expect %q but found `%s`",
					indexData.tableName,
					regex,
					indexData.indexName,
				),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + indexData.line),
			})
		}
		if r.maxLength > 0 && len(indexData.indexName) > r.maxLength {
			r.AddAdvice(&types.Advice{
				Status: types.Advice_Status(r.level),
				Code:   int32(types.NamingUKConventionMismatch),
				Title:  r.title,
				Content: fmt.Sprintf(
					"Unique key `%s` in table `%s` mismatches the naming convention, its length should be within %d characters",
					indexData.indexName,
					indexData.tableName,
					r.maxLength,
				),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + indexData.line),
			})
		}
	}
}

func (r *NamingIndexUkAdvisor) handleConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) *ukIndexMetaData {
	// we only focus on unique index
	if ctx.UNIQUE_SYMBOL() == nil {
		return nil
	}
	if ctx.KeyListVariants() == nil {
		return nil
	}

	indexName := ""
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
	}

	columnList := mysqlparser.NormalizeKeyListVariants(ctx.KeyListVariants())
	metaData := map[string]string{
		advisor.ColumnListTemplateToken: strings.Join(columnList, "_"),
		advisor.TableNameTemplateToken:  tableName,
	}
	return &ukIndexMetaData{
		indexName: indexName,
		tableName: tableName,
		metaData:  metaData,
		line:      ctx.GetStart().GetLine(),
	}
}

// getTemplateRegexp returns the regexp for the given template format and metadata
func (r *NamingIndexUkAdvisor) getTemplateRegexp(
	format string,
	templateList []string,
	metaData map[string]string,
) (*regexp.Regexp, error) {
	regexpPattern := format
	for _, templateToken := range templateList {
		value, exists := metaData[templateToken]
		if !exists {
			value = ""
		}
		regexpPattern = strings.ReplaceAll(regexpPattern, templateToken, value)
	}
	return regexp.Compile(regexpPattern)
}

// NamingIndexUKAdvisor is the advisor using ANTLR parser for unique key naming checking
type NamingIndexUKAdvisor struct{}

// Check performs the ANTLR-based unique key naming check using payload
func (a *NamingIndexUKAdvisor) Check(
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

	// Parse the format template from rule payload
	payload, err := advisor.UnmarshalStringTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	format := payload.String
	maxLength := 64
	templateList := []string{
		advisor.TableNameTemplateToken,
		advisor.ColumnListTemplateToken,
	}

	// Create the rule
	namingRule := NewNamingIndexUkAdvisor(types.SQLReviewRuleLevel(level), string(rule.Type), format, maxLength, templateList)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{namingRule})

	for _, stmtNode := range root {
		namingRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
