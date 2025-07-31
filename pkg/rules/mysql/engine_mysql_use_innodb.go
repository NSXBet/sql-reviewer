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

const (
	innoDB              string = "innodb"
	defaultStorageEngin string = "default_storage_engine"
)

// UseInnoDBRule is the ANTLR-based implementation for checking InnoDB engine usage
type UseInnoDBRule struct {
	BaseAntlrRule
}

// NewUseInnoDBRule creates a new ANTLR-based InnoDB engine rule
func NewUseInnoDBRule(level types.SQLReviewRuleLevel, title string) *UseInnoDBRule {
	return &UseInnoDBRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*UseInnoDBRule) Name() string {
	return "UseInnoDBRule"
}

// OnEnter is called when entering a parse tree node
func (r *UseInnoDBRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeSetStatement:
		r.checkSetStatement(ctx.(*mysql.SetStatementContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*UseInnoDBRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *UseInnoDBRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.CreateTableOptions() == nil {
		return
	}
	for _, tableOption := range ctx.CreateTableOptions().AllCreateTableOption() {
		if tableOption.ENGINE_SYMBOL() != nil && tableOption.EngineRef() != nil {
			if tableOption.EngineRef().TextOrIdentifier() == nil {
				continue
			}
			engine := mysqlparser.NormalizeMySQLTextOrIdentifier(tableOption.EngineRef().TextOrIdentifier())
			if strings.ToLower(engine) != innoDB {
				content := "CREATE " + ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
				line := tableOption.GetStart().GetLine()
				r.addAdvice(content, line)
				break
			}
		}
	}
}

func (r *UseInnoDBRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}
	needsAdvice := false
	for _, option := range ctx.AlterTableActions().AlterCommandList().AlterList().AllCreateTableOptionsSpaceSeparated() {
		for _, op := range option.AllCreateTableOption() {
			if op.ENGINE_SYMBOL() != nil {
				if op.EngineRef() == nil {
					continue
				}
				engine := op.EngineRef().GetText()
				if strings.ToLower(engine) != innoDB {
					needsAdvice = true
					break
				}
			}
		}
	}

	if needsAdvice {
		content := "ALTER " + ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
		line := ctx.GetStart().GetLine()
		r.addAdvice(content, line)
	}
}

func (r *UseInnoDBRule) checkSetStatement(ctx *mysql.SetStatementContext) {
	needsAdvice := false
	if ctx.StartOptionValueList() == nil {
		return
	}

	startOptionValueList := ctx.StartOptionValueList()
	if startOptionValueList.OptionValueNoOptionType() == nil {
		return
	}
	optionValueNoOptionType := startOptionValueList.OptionValueNoOptionType()
	if optionValueNoOptionType.InternalVariableName() == nil {
		return
	}
	name := optionValueNoOptionType.InternalVariableName().GetText()
	if strings.ToLower(name) != defaultStorageEngin {
		return
	}
	if optionValueNoOptionType.SetExprOrDefault() != nil {
		engine := optionValueNoOptionType.SetExprOrDefault().GetText()
		if strings.ToLower(engine) != innoDB {
			needsAdvice = true
		}
	}

	if needsAdvice {
		content := ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
		line := ctx.GetStart().GetLine()
		r.addAdvice(content, line)
	}
}

func (r *UseInnoDBRule) addAdvice(content string, lineNumber int) {
	lineNumber += r.baseLine
	r.AddAdvice(&types.Advice{
		Status:        types.Advice_Status(r.level),
		Code:          int32(types.NotInnoDBEngine),
		Title:         r.title,
		Content:       fmt.Sprintf("\"%s;\" doesn't use InnoDB engine", content),
		StartPosition: ConvertANTLRLineToPosition(lineNumber),
	})
}

// EngineMySQLUseInnoDBAdvisor is the advisor using ANTLR parser for InnoDB engine checking
type EngineMySQLUseInnoDBAdvisor struct{}

// Check performs the ANTLR-based InnoDB engine check
func (a *EngineMySQLUseInnoDBAdvisor) Check(
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
	engineRule := NewUseInnoDBRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{engineRule})

	for _, stmtNode := range root {
		engineRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
