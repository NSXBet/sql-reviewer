package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*TableNoFKAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleTableNoFK), &TableNoFKAdvisor{})
}

type TableNoFKAdvisor struct{}

func (*TableNoFKAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &tableNoFKChecker{
		level:          level,
		title:          string(checkCtx.Rule.Type),
		statementsText: checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type tableNoFKChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList     []*types.Advice
	level          types.Advice_Status
	title          string
	statementsText string
}

func (c *tableNoFKChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	var tableName, schemaName string
	allQualifiedNames := ctx.AllQualified_name()
	if len(allQualifiedNames) > 0 {
		tableName = extractTableName(allQualifiedNames[0])
		schemaName = extractSchemaName(allQualifiedNames[0])
		if schemaName == "" {
			schemaName = "public"
		}
	}

	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.Tableconstraint() != nil {
				constraint := elem.Tableconstraint()
				if constraint.Constraintelem() != nil {
					constraintElem := constraint.Constraintelem()
					if constraintElem.FOREIGN() != nil && constraintElem.KEY() != nil {
						c.addFKAdvice(schemaName, tableName, ctx)
						return
					}
				}
			}

			if elem.ColumnDef() != nil {
				columnDef := elem.ColumnDef()
				if columnDef.Colquallist() != nil {
					allQuals := columnDef.Colquallist().AllColconstraint()
					for _, qual := range allQuals {
						if qual.Colconstraintelem() != nil {
							constraintElem := qual.Colconstraintelem()
							if constraintElem.REFERENCES() != nil {
								c.addFKAdvice(schemaName, tableName, ctx)
								return
							}
						}
					}
				}
			}
		}
	}
}

func (c *tableNoFKChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	var tableName, schemaName string
	if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
		tableName = extractTableName(ctx.Relation_expr().Qualified_name())
		schemaName = extractSchemaName(ctx.Relation_expr().Qualified_name())
		if schemaName == "" {
			schemaName = "public"
		}
	}

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
				constraint := cmd.Tableconstraint()
				if constraint.Constraintelem() != nil {
					constraintElem := constraint.Constraintelem()
					if constraintElem.FOREIGN() != nil && constraintElem.KEY() != nil {
						c.addFKAdvice(schemaName, tableName, ctx)
						return
					}
				}
			}
		}
	}
}

func (c *tableNoFKChecker) addFKAdvice(schemaName, tableName string, ctx antlr.ParserRuleContext) {
	stmtText := extractStatementTextForNoFK(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
	c.adviceList = append(c.adviceList, &types.Advice{
		Status: c.level,
		Code:   int32(types.TableHasFK),
		Title:  c.title,
		Content: fmt.Sprintf(
			"Foreign key is not allowed in the table %q.%q, related statement: \"%s\"",
			schemaName,
			tableName,
			stmtText,
		),
		StartPosition: &types.Position{
			Line: int32(ctx.GetStart().GetLine()),
		},
	})
}

func extractStatementTextForNoFK(statementsText string, startLine, endLine int) string {
	lines := strings.Split(statementsText, "\n")
	if startLine < 1 || startLine > len(lines) {
		return ""
	}

	startIdx := startLine - 1
	endIdx := endLine - 1

	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	var stmtLines []string
	for i := startIdx; i <= endIdx; i++ {
		stmtLines = append(stmtLines, lines[i])
	}

	return strings.TrimSpace(strings.Join(stmtLines, " "))
}
