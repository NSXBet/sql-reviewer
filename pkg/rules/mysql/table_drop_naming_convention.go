package mysql

import (
	"context"
	"fmt"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// TableDropNamingConventionRule is the ANTLR-based implementation for checking table drop naming convention
type TableDropNamingConventionRule struct {
	BaseAntlrRule
	format *regexp.Regexp
}

// NewTableDropNamingConventionRule creates a new ANTLR-based table drop naming convention rule
func NewTableDropNamingConventionRule(
	level types.SQLReviewRuleLevel,
	title string,
	format *regexp.Regexp,
) *TableDropNamingConventionRule {
	return &TableDropNamingConventionRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		format: format,
	}
}

// Name returns the rule name
func (*TableDropNamingConventionRule) Name() string {
	return "TableDropNamingConventionRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableDropNamingConventionRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeDropTable {
		r.checkDropTable(ctx.(*mysql.DropTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableDropNamingConventionRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableDropNamingConventionRule) checkDropTable(ctx *mysql.DropTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableRefList() == nil {
		return
	}

	for _, tableRef := range ctx.TableRefList().AllTableRef() {
		_, tableName := mysqlparser.NormalizeMySQLTableRef(tableRef)
		if !r.format.MatchString(tableName) {
			r.AddAdvice(&types.Advice{
				Status: types.Advice_Status(r.level),
				Code:   int32(types.TableDropNamingConventionMismatch),
				Title:  r.title,
				Content: fmt.Sprintf(
					"`%s` mismatches drop table naming convention, naming format should be %q",
					tableName,
					r.format,
				),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// TableDropNamingConventionAdvisor is the advisor using ANTLR parser for table drop naming convention checking
type TableDropNamingConventionAdvisor struct{}

// Check performs the ANTLR-based table drop naming convention check using payload
func (a *TableDropNamingConventionAdvisor) Check(
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

	// Parse the regex pattern from rule payload
	payload, err := advisor.UnmarshalStringTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	format, err := regexp.Compile(payload.String)
	if err != nil {
		return nil, fmt.Errorf("invalid regex format: %w", err)
	}

	// Create the rule
	dropRule := NewTableDropNamingConventionRule(types.SQLReviewRuleLevel(level), string(rule.Type), format)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{dropRule})

	for _, stmtNode := range root {
		dropRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
