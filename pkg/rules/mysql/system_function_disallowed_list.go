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

// SystemFunctionDisallowedListRule is the ANTLR-based implementation for checking disallowed function list
type SystemFunctionDisallowedListRule struct {
	BaseAntlrRule
	text         string
	disallowList []string
}

// NewSystemFunctionDisallowedListRule creates a new ANTLR-based system function disallowed list rule
func NewSystemFunctionDisallowedListRule(level types.SQLReviewRuleLevel, title string, disallowList []string) *SystemFunctionDisallowedListRule {
	return &SystemFunctionDisallowedListRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		disallowList: disallowList,
	}
}

// Name returns the rule name
func (*SystemFunctionDisallowedListRule) Name() string {
	return "SystemFunctionDisallowedListRule"
}

// OnEnter is called when entering a parse tree node
func (r *SystemFunctionDisallowedListRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		r.checkQuery(ctx.(*mysql.QueryContext))
	case NodeTypeFunctionCall:
		r.checkFunctionCall(ctx.(*mysql.FunctionCallContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*SystemFunctionDisallowedListRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *SystemFunctionDisallowedListRule) checkQuery(ctx *mysql.QueryContext) {
	r.text = ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
}

func (r *SystemFunctionDisallowedListRule) checkFunctionCall(ctx *mysql.FunctionCallContext) {
	pi := ctx.PureIdentifier()
	if pi != nil {
		functionName := mysqlparser.NormalizeMySQLPureIdentifier(pi)
		if slices.Contains(r.disallowList, strings.ToUpper(functionName)) {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.DisabledFunction),
				Title:         r.title,
				Content:       fmt.Sprintf("Function \"%s\" is disallowed, but \"%s\" uses", functionName, r.text),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

// SystemFunctionDisallowedListAdvisor is the advisor using ANTLR parser for system function disallowed list checking
type SystemFunctionDisallowedListAdvisor struct{}

// Check performs the ANTLR-based system function disallowed list check
func (a *SystemFunctionDisallowedListAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Parse the function list from rule payload
	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	
	var disallowList []string
	for _, fn := range payload.List {
		disallowList = append(disallowList, strings.ToUpper(strings.TrimSpace(fn)))
	}

	// Create the rule
	functionRule := NewSystemFunctionDisallowedListRule(types.SQLReviewRuleLevel(level), string(rule.Type), disallowList)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{functionRule})

	for _, stmtNode := range root {
		functionRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}