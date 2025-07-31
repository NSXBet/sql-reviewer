package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// StatementAddColumnWithoutPositionRule is the ANTLR-based implementation for checking no position in ADD COLUMN clause
type StatementAddColumnWithoutPositionRule struct {
	BaseAntlrRule
}

// NewStatementAddColumnWithoutPositionRule creates a new ANTLR-based statement add column without position rule
func NewStatementAddColumnWithoutPositionRule(
	level types.SQLReviewRuleLevel,
	title string,
) *StatementAddColumnWithoutPositionRule {
	return &StatementAddColumnWithoutPositionRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*StatementAddColumnWithoutPositionRule) Name() string {
	return "StatementAddColumnWithoutPositionRule"
}

// OnEnter is called when entering a parse tree node
func (r *StatementAddColumnWithoutPositionRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeAlterTable {
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*StatementAddColumnWithoutPositionRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *StatementAddColumnWithoutPositionRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
	if tableName == "" {
		return
	}

	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil || item.ADD_SYMBOL() == nil {
			continue
		}

		var position string

		switch {
		case item.Identifier() != nil && item.FieldDefinition() != nil:
			position = r.getPosition(item.Place())
		case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
			for _, tableElement := range item.TableElementList().AllTableElement() {
				if tableElement.ColumnDefinition() == nil {
					continue
				}
				if tableElement.ColumnDefinition().FieldDefinition() == nil {
					continue
				}

				position = r.getPosition(item.Place())
				if len(position) != 0 {
					break
				}
			}
		}

		if len(position) != 0 {
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.StatementAddColumnWithPosition),
				Title:         r.title,
				Content:       fmt.Sprintf("add column with position \"%s\"", position),
				StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
			})
		}
	}
}

func (r *StatementAddColumnWithoutPositionRule) getPosition(ctx mysql.IPlaceContext) string {
	if ctx == nil {
		return ""
	}
	place, ok := ctx.(*mysql.PlaceContext)
	if !ok || place == nil {
		return ""
	}

	switch {
	case place.FIRST_SYMBOL() != nil:
		return "FIRST"
	case place.AFTER_SYMBOL() != nil:
		return "AFTER"
	default:
		return ""
	}
}

// StatementAddColumnWithoutPositionAdvisor is the advisor using ANTLR parser for statement add column without position checking
type StatementAddColumnWithoutPositionAdvisor struct{}

// Check performs the ANTLR-based statement add column without position check
func (a *StatementAddColumnWithoutPositionAdvisor) Check(
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
	statementAddColumnWithoutPositionRule := NewStatementAddColumnWithoutPositionRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{statementAddColumnWithoutPositionRule})

	for _, stmtNode := range root {
		statementAddColumnWithoutPositionRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
