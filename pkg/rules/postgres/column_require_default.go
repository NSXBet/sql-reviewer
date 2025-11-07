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

var _ advisor.Advisor = (*ColumnRequireDefaultAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleColumnRequireDefault), &ColumnRequireDefaultAdvisor{})
}

type ColumnRequireDefaultAdvisor struct{}

func (*ColumnRequireDefaultAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &columnRequireDefaultChecker{
		level: level,
		title: string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type columnRequireDefaultChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
}

func (c *columnRequireDefaultChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
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
				if colDef.Colid() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					if !c.hasDefault(colDef) {
						c.adviceList = append(c.adviceList, &types.Advice{
							Status: c.level,
							Code:   int32(types.ColumnRequireDefault),
							Title:  c.title,
							Content: fmt.Sprintf(
								"Column %q.%q in schema %q doesn't have DEFAULT",
								tableName,
								columnName,
								"public",
							),
							StartPosition: &types.Position{
								Line: int32(colDef.GetStart().GetLine()),
							},
						})
					}
				}
			}
		}
	}
}

func (c *columnRequireDefaultChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
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
				if colDef.Colid() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					if !c.hasDefault(colDef) {
						c.adviceList = append(c.adviceList, &types.Advice{
							Status: c.level,
							Code:   int32(types.ColumnRequireDefault),
							Title:  c.title,
							Content: fmt.Sprintf(
								"Column %q.%q in schema %q doesn't have DEFAULT",
								tableName,
								columnName,
								"public",
							),
							StartPosition: &types.Position{
								Line: int32(colDef.GetStart().GetLine()),
							},
						})
					}
				}
			}
		}
	}
}

func (*columnRequireDefaultChecker) hasDefault(colDef parser.IColumnDefContext) bool {
	if colDef.Typename() != nil && colDef.Typename().Simpletypename() != nil {
		simpleType := colDef.Typename().Simpletypename()
		typeText := strings.ToLower(simpleType.GetText())
		if typeText == "serial" || typeText == "bigserial" || typeText == "smallserial" {
			return true
		}
	}

	if colDef.Colquallist() == nil {
		return false
	}

	allConstraints := colDef.Colquallist().AllColconstraint()
	for _, constraint := range allConstraints {
		if constraint.Colconstraintelem() == nil {
			continue
		}

		elem := constraint.Colconstraintelem()

		if elem.DEFAULT() != nil {
			return true
		}
	}

	return false
}
