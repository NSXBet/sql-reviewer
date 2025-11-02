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

// TableDisallowPartitionRule is the ANTLR-based implementation for checking disallowed table partitions
type TableDisallowPartitionRule struct {
	BaseAntlrRule
	text string
}

// NewTableDisallowPartitionRule creates a new ANTLR-based table disallow partition rule
func NewTableDisallowPartitionRule(level types.SQLReviewRuleLevel, title string) *TableDisallowPartitionRule {
	return &TableDisallowPartitionRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*TableDisallowPartitionRule) Name() string {
	return "TableDisallowPartitionRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableDisallowPartitionRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableDisallowPartitionRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableDisallowPartitionRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.PartitionClause() != nil && ctx.PartitionClause().PartitionTypeDef() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.CreateTablePartition),
			Title:         r.title,
			Content:       fmt.Sprintf("Table partition is forbidden, but \"%s\" creates", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *TableDisallowPartitionRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.AlterTableActions() != nil && ctx.AlterTableActions().PartitionClause() != nil &&
		ctx.AlterTableActions().PartitionClause().PartitionTypeDef() != nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.CreateTablePartition),
			Title:         r.title,
			Content:       fmt.Sprintf("Table partition is forbidden, but \"%s\" creates", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// TableDisallowPartitionAdvisor is the advisor using ANTLR parser for table disallow partition checking
type TableDisallowPartitionAdvisor struct{}

// Check performs the ANTLR-based table disallow partition check
func (a *TableDisallowPartitionAdvisor) Check(
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

	// Create the rule
	partitionRule := NewTableDisallowPartitionRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{partitionRule})

	for _, stmtNode := range root {
		partitionRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
