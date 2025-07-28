package mysql

import (
	"context"
	"fmt"
	"strconv"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ColumnAutoIncrementInitialValueRule is the ANTLR-based implementation for checking auto-increment column initial value
type ColumnAutoIncrementInitialValueRule struct {
	BaseAntlrRule
	value int
}

// NewColumnAutoIncrementInitialValueRule creates a new ANTLR-based auto-increment column initial value rule
func NewColumnAutoIncrementInitialValueRule(
	level types.SQLReviewRuleLevel,
	title string,
	value int,
) *ColumnAutoIncrementInitialValueRule {
	return &ColumnAutoIncrementInitialValueRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		value: value,
	}
}

// Name returns the rule name
func (*ColumnAutoIncrementInitialValueRule) Name() string {
	return "ColumnAutoIncrementInitialValueRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnAutoIncrementInitialValueRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnAutoIncrementInitialValueRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnAutoIncrementInitialValueRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.CreateTableOptions() == nil || ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, option := range ctx.CreateTableOptions().AllCreateTableOption() {
		if option.AUTO_INCREMENT_SYMBOL() == nil || option.Ulonglong_number() == nil {
			continue
		}

		base := 10
		bitSize := 0
		value, err := strconv.ParseUint(option.Ulonglong_number().GetText(), base, bitSize)
		if err != nil {
			continue
		}
		if value != uint64(r.value) {
			r.AddAdvice(&types.Advice{
				Status: types.Advice_Status(r.level),
				Code:   int32(types.AutoIncrementInitialValueNotMatch),
				Title:  r.title,
				Content: fmt.Sprintf(
					"The initial auto-increment value in table `%s` is %v, which doesn't equal %v",
					tableName,
					value,
					r.value,
				),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

func (r *ColumnAutoIncrementInitialValueRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	if tableName == "" {
		return
	}

	// alter table option.
	for _, option := range ctx.AlterTableActions().AlterCommandList().AlterList().AllCreateTableOptionsSpaceSeparated() {
		if option == nil {
			continue
		}
		for _, tableOption := range option.AllCreateTableOption() {
			if tableOption == nil || tableOption.AUTO_INCREMENT_SYMBOL() == nil || tableOption.Ulonglong_number() == nil {
				continue
			}

			base := 10
			bitSize := 0
			value, err := strconv.ParseUint(tableOption.Ulonglong_number().GetText(), base, bitSize)
			if err != nil {
				continue
			}
			if value != uint64(r.value) {
				r.AddAdvice(&types.Advice{
					Status: types.Advice_Status(r.level),
					Code:   int32(types.AutoIncrementInitialValueNotMatch),
					Title:  r.title,
					Content: fmt.Sprintf(
						"The initial auto-increment value in table `%s` is %v, which doesn't equal %v",
						tableName,
						value,
						r.value,
					),
					StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
				})
			}
		}
	}
}

// ColumnAutoIncrementInitialValueAdvisor is the advisor using ANTLR parser for auto-increment column initial value checking
type ColumnAutoIncrementInitialValueAdvisor struct{}

// Check performs the ANTLR-based auto-increment column initial value check
func (a *ColumnAutoIncrementInitialValueAdvisor) Check(
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

	// Parse the numeric parameter from rule payload
	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}
	value := int(payload.Number)

	// Create the rule
	autoIncrementRule := NewColumnAutoIncrementInitialValueRule(types.SQLReviewRuleLevel(level), string(rule.Type), value)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{autoIncrementRule})

	for _, stmtNode := range root {
		autoIncrementRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
