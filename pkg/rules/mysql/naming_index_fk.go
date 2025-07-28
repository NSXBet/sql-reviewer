package mysql

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// fkIndexMetaData is the metadata for foreign key.
type fkIndexMetaData struct {
	indexName string
	tableName string
	metaData  map[string]string
	line      int
}

// NamingIndexFkAdvisor is the ANTLR-based implementation for checking foreign key naming convention
type NamingIndexFkAdvisor struct {
	BaseAntlrRule
	format       string
	maxLength    int
	templateList []string
}

// NewNamingIndexFkAdvisor creates a new ANTLR-based foreign key naming rule
func NewNamingIndexFkAdvisor(level types.SQLReviewRuleLevel, title string, format string, maxLength int, templateList []string) *NamingIndexFkAdvisor {
	return &NamingIndexFkAdvisor{
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
func (*NamingIndexFkAdvisor) Name() string {
	return "NamingIndexFkAdvisor"
}

// OnEnter is called when entering a parse tree node
func (r *NamingIndexFkAdvisor) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*NamingIndexFkAdvisor) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *NamingIndexFkAdvisor) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())

	var indexDataList []*fkIndexMetaData
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

func (r *NamingIndexFkAdvisor) checkAlterTable(ctx *mysql.AlterTableContext) {
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
	var indexDataList []*fkIndexMetaData
	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if alterListItem == nil {
			continue
		}

		// add constraint.
		if alterListItem.ADD_SYMBOL() != nil && alterListItem.TableConstraintDef() != nil {
			if metaData := r.handleConstraintDef(tableName, alterListItem.TableConstraintDef()); metaData != nil {
				indexDataList = append(indexDataList, metaData)
			}
		}
	}
	r.handleIndexList(indexDataList)
}

func (r *NamingIndexFkAdvisor) handleIndexList(indexDataList []*fkIndexMetaData) {
	for _, indexData := range indexDataList {
		regex, err := r.getTemplateRegexp(r.format, r.templateList, indexData.metaData)
		if err != nil {
			r.AddAdvice(&types.Advice{
				Status:  types.Advice_Status(r.level),
				Code:    int32(types.Internal),
				Title:   "Internal error for foreign key naming convention rule",
				Content: fmt.Sprintf("Internal error: %v", err),
			})
			continue
		}
		if !regex.MatchString(indexData.indexName) {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.NamingFKConventionMismatch),
				Title:         r.title,
				Content:       fmt.Sprintf("Foreign key in table `%s` mismatches the naming convention, expect %q but found `%s`", indexData.tableName, regex, indexData.indexName),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + indexData.line),
			})
		}
		if r.maxLength > 0 && len(indexData.indexName) > r.maxLength {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.NamingFKConventionMismatch),
				Title:         r.title,
				Content:       fmt.Sprintf("Foreign key `%s` in table `%s` mismatches the naming convention, its length should be within %d characters", indexData.indexName, indexData.tableName, r.maxLength),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + indexData.line),
			})
		}
	}
}

func (r *NamingIndexFkAdvisor) handleConstraintDef(tableName string, ctx mysql.ITableConstraintDefContext) *fkIndexMetaData {
	// focus on foreign index.
	if ctx.FOREIGN_SYMBOL() == nil || ctx.KEY_SYMBOL() == nil || ctx.KeyList() == nil || ctx.References() == nil {
		return nil
	}

	indexName := ""
	// for compatibility.
	if ctx.IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexName())
	}
	// use constraint_name if both exist at the same time.
	// for mysql, foreign key use constraint name as unique identifier.
	if ctx.ConstraintName() != nil {
		indexName = mysqlparser.NormalizeConstraintName(ctx.ConstraintName())
	}

	referencingColumnList := mysqlparser.NormalizeKeyList(ctx.KeyList())
	referencedTable, referencedColumnList := r.handleReferences(ctx.References())
	metaData := map[string]string{
		advisor.ReferencingTableNameTemplateToken:  tableName,
		advisor.ReferencingColumnNameTemplateToken: strings.Join(referencingColumnList, "_"),
		advisor.ReferencedTableNameTemplateToken:   referencedTable,
		advisor.ReferencedColumnNameTemplateToken:  strings.Join(referencedColumnList, "_"),
	}
	return &fkIndexMetaData{
		indexName: indexName,
		tableName: tableName,
		metaData:  metaData,
		line:      ctx.GetStart().GetLine(),
	}
}

func (*NamingIndexFkAdvisor) handleReferences(ctx mysql.IReferencesContext) (string, []string) {
	tableName := ""
	if ctx.TableRef() != nil {
		_, tableName = mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	}

	var columns []string
	if ctx.IdentifierListWithParentheses() != nil {
		columns = mysqlparser.NormalizeIdentifierListWithParentheses(ctx.IdentifierListWithParentheses())
	}
	return tableName, columns
}

// getTemplateRegexp returns the regexp for the given template format and metadata
func (r *NamingIndexFkAdvisor) getTemplateRegexp(format string, templateList []string, metaData map[string]string) (*regexp.Regexp, error) {
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

// NamingIndexFKAdvisor is the advisor using ANTLR parser for foreign key naming checking
type NamingIndexFKAdvisor struct{}

// Check performs the ANTLR-based foreign key naming check using payload
func (a *NamingIndexFKAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
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
		advisor.ReferencingTableNameTemplateToken,
		advisor.ReferencingColumnNameTemplateToken,
		advisor.ReferencedTableNameTemplateToken,
		advisor.ReferencedColumnNameTemplateToken,
	}

	// Create the rule
	namingRule := NewNamingIndexFkAdvisor(types.SQLReviewRuleLevel(level), string(rule.Type), format, maxLength, templateList)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{namingRule})

	for _, stmtNode := range root {
		namingRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
