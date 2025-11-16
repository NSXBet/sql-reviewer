package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnTypeDisallowListAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleColumnTypeDisallowList),
		&ColumnTypeDisallowListAdvisor{},
	)
}

type ColumnTypeDisallowListAdvisor struct{}

func (*ColumnTypeDisallowListAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	typeRestriction := make(map[string]bool)
	for _, tp := range payload.List {
		typeRestriction[strings.ToLower(tp)] = true
	}

	checker := &columnTypeDisallowListChecker{
		level:           level,
		title:           string(checkCtx.Rule.Type),
		typeRestriction: typeRestriction,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type columnTypeDisallowListChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList      []*types.Advice
	level           types.Advice_Status
	title           string
	typeRestriction map[string]bool
}

func (c *columnTypeDisallowListChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	qualifiedNames := ctx.AllQualified_name()
	if len(qualifiedNames) == 0 {
		return
	}

	tableName := extractTableName(qualifiedNames[0])
	if tableName == "" {
		return
	}

	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.ColumnDef() != nil {
				colDef := elem.ColumnDef()
				if colDef.Colid() != nil && colDef.Typename() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					c.checkType(tableName, columnName, colDef.Typename(), colDef.GetStart().GetLine())
				}
			}
		}
	}
}

func (c *columnTypeDisallowListChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}

	tableName := extractTableName(ctx.Relation_expr().Qualified_name())
	if tableName == "" {
		return
	}

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
				colDef := cmd.ColumnDef()
				if colDef.Colid() != nil && colDef.Typename() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					c.checkType(tableName, columnName, colDef.Typename(), colDef.GetStart().GetLine())
				}
			}

			if cmd.ALTER() != nil && cmd.TYPE_P() != nil && cmd.Typename() != nil {
				allColids := cmd.AllColid()
				if len(allColids) > 0 {
					columnName := pgparser.NormalizePostgreSQLColid(allColids[0])
					c.checkType(tableName, columnName, cmd.Typename(), cmd.GetStart().GetLine())
				}
			}
		}
	}
}

func (c *columnTypeDisallowListChecker) checkType(tableName, columnName string, typename parser.ITypenameContext, line int) {
	if typename == nil {
		return
	}

	typeText := typename.GetText()

	var matchedDisallowedType string
	for disallowedType := range c.typeRestriction {
		if areTypesEquivalent(typeText, disallowedType) {
			matchedDisallowedType = disallowedType
			break
		}
	}

	if matchedDisallowedType != "" {
		c.adviceList = append(c.adviceList, &types.Advice{
			Status: c.level,
			Code:   int32(types.DisabledColumnType),
			Title:  c.title,
			Content: fmt.Sprintf(
				"Disallow column type %s but column %q.%q is",
				strings.ToUpper(matchedDisallowedType),
				tableName,
				columnName,
			),
			StartPosition: &types.Position{
				Line: int32(line),
			},
		})
	}
}
