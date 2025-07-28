package mysql

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// IndexTypeAllowListRule is the ANTLR-based implementation for checking index type allow list
type IndexTypeAllowListRule struct {
	BaseAntlrRule
	allowList []string
}

// NewIndexTypeAllowListRule creates a new ANTLR-based index type allow list rule
func NewIndexTypeAllowListRule(level types.SQLReviewRuleLevel, title string, allowList []string) *IndexTypeAllowListRule {
	return &IndexTypeAllowListRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		allowList: allowList,
	}
}

// Name returns the rule name
func (*IndexTypeAllowListRule) Name() string {
	return "IndexTypeAllowListRule"
}

// OnEnter is called when entering a parse tree node
func (r *IndexTypeAllowListRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
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
func (*IndexTypeAllowListRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *IndexTypeAllowListRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil || tableElement.TableConstraintDef() == nil {
			continue
		}
		r.handleConstraintDef(tableElement.TableConstraintDef())
	}
}

func (r *IndexTypeAllowListRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.TableRef() == nil {
		return
	}
	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil || ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	for _, alterListItem := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if alterListItem == nil {
			continue
		}
		if alterListItem.ADD_SYMBOL() != nil && alterListItem.TableConstraintDef() != nil {
			r.handleConstraintDef(alterListItem.TableConstraintDef())
		}
	}
}

func (r *IndexTypeAllowListRule) handleConstraintDef(ctx mysql.ITableConstraintDefContext) {
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserINDEX_SYMBOL, mysql.MySQLParserKEY_SYMBOL, mysql.MySQLParserPRIMARY_SYMBOL, mysql.MySQLParserUNIQUE_SYMBOL, mysql.MySQLParserFULLTEXT_SYMBOL, mysql.MySQLParserSPATIAL_SYMBOL:
	default:
		return
	}

	indexType := "BTREE"
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexType() != nil {
		indexType = ctx.IndexNameAndType().IndexType().GetText()
	} else {
		if ctx.FULLTEXT_SYMBOL() != nil {
			indexType = "FULLTEXT"
		} else if ctx.SPATIAL_SYMBOL() != nil {
			indexType = "SPATIAL"
		}
	}
	r.validateIndexType(indexType, ctx.GetStart().GetLine())
}

func (r *IndexTypeAllowListRule) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil || ctx.CreateIndexTarget().KeyListVariants() == nil {
		return
	}

	indexType := "BTREE"
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexType() != nil {
		indexType = ctx.IndexNameAndType().IndexType().GetText()
	} else {
		if ctx.FULLTEXT_SYMBOL() != nil {
			indexType = "FULLTEXT"
		} else if ctx.SPATIAL_SYMBOL() != nil {
			indexType = "SPATIAL"
		}
	}
	r.validateIndexType(indexType, ctx.GetStart().GetLine())
}

// validateIndexType checks if the index type is in the allow list.
func (r *IndexTypeAllowListRule) validateIndexType(indexType string, line int) {
	if slices.Contains(r.allowList, indexType) {
		return
	}

	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.IndexTypeNotAllowed),
		Title:         r.title,
		Content:       fmt.Sprintf("Index type `%s` is not allowed", indexType),
		StartPosition: ConvertANTLRLineToPosition(r.baseLine + line),
	})
}

// IndexTypeAllowListAdvisor is the advisor using ANTLR parser for index type allow list checking
type IndexTypeAllowListAdvisor struct{}

// Check performs the ANTLR-based index type allow list check using payload
func (a *IndexTypeAllowListAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Parse the allowlist from rule payload
	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	var allowList []string
	for _, typeStr := range payload.List {
		typeStr = strings.TrimSpace(typeStr)
		if typeStr != "" {
			allowList = append(allowList, typeStr)
		}
	}

	// Create the rule with allowlist
	indexTypeAllowRule := NewIndexTypeAllowListRule(types.SQLReviewRuleLevel(level), string(rule.Type), allowList)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{indexTypeAllowRule})

	for _, stmtNode := range root {
		indexTypeAllowRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}