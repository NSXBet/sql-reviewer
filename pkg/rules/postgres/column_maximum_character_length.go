package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnMaximumCharacterLengthAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleColumnMaximumCharacterLength),
		&ColumnMaximumCharacterLengthAdvisor{},
	)
}

type ColumnMaximumCharacterLengthAdvisor struct{}

func (*ColumnMaximumCharacterLengthAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	if payload.Number <= 0 {
		return nil, nil
	}

	checker := &columnMaximumCharacterLengthChecker{
		level:   level,
		title:   string(checkCtx.Rule.Type),
		maximum: payload.Number,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type columnMaximumCharacterLengthChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	maximum    int
}

func (c *columnMaximumCharacterLengthChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	tableName := c.extractTableName(ctx.AllQualified_name())

	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.ColumnDef() != nil {
				colDef := elem.ColumnDef()
				if colDef.Colid() != nil && colDef.Typename() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					charLength := c.getCharLength(colDef.Typename())
					if charLength > c.maximum {
						c.addAdvice(tableName, columnName, colDef.GetStart().GetLine())
						return
					}
				}
			}
		}
	}
}

func (c *columnMaximumCharacterLengthChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}

	parts := pgparser.NormalizePostgreSQLQualifiedName(ctx.Relation_expr().Qualified_name())
	if len(parts) == 0 {
		return
	}

	var tableName string
	if len(parts) == 1 {
		tableName = fmt.Sprintf("%q", parts[0])
	} else {
		tableName = fmt.Sprintf("%q.%q", parts[0], parts[1])
	}

	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			if cmd.ADD_P() != nil && cmd.ColumnDef() != nil {
				colDef := cmd.ColumnDef()
				if colDef.Colid() != nil && colDef.Typename() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					charLength := c.getCharLength(colDef.Typename())
					if charLength > c.maximum {
						c.addAdvice(tableName, columnName, colDef.GetStart().GetLine())
						return
					}
				}
			}

			if cmd.ALTER() != nil && cmd.TYPE_P() != nil && cmd.Typename() != nil {
				allColids := cmd.AllColid()
				if len(allColids) > 0 {
					columnName := pgparser.NormalizePostgreSQLColid(allColids[0])
					charLength := c.getCharLength(cmd.Typename())
					if charLength > c.maximum {
						c.addAdvice(tableName, columnName, cmd.GetStart().GetLine())
						return
					}
				}
			}
		}
	}
}

func (*columnMaximumCharacterLengthChecker) extractTableName(qualifiedNames []parser.IQualified_nameContext) string {
	if len(qualifiedNames) == 0 {
		return ""
	}

	parts := pgparser.NormalizePostgreSQLQualifiedName(qualifiedNames[0])
	if len(parts) == 0 {
		return ""
	}

	if len(parts) == 1 {
		return fmt.Sprintf("%q", parts[0])
	}
	return fmt.Sprintf("%q.%q", parts[0], parts[1])
}

func (*columnMaximumCharacterLengthChecker) getCharLength(typename parser.ITypenameContext) int {
	if typename == nil {
		return 0
	}

	if typename.Simpletypename() == nil {
		return 0
	}

	simpleType := typename.Simpletypename()

	if simpleType.Character() == nil {
		return 0
	}

	character := simpleType.Character()
	if character.Character_c() == nil {
		return 0
	}

	characterC := character.Character_c()

	if characterC.VARCHAR() != nil {
		return 0
	}

	if (characterC.CHARACTER() != nil || characterC.CHAR_P() != nil || characterC.NCHAR() != nil) &&
		characterC.Opt_varying() != nil {
		return 0
	}

	if character.Iconst() != nil {
		size, err := extractIntegerConstant(character.Iconst())
		if err != nil {
			return 0
		}
		return size
	}

	return 0
}

func (c *columnMaximumCharacterLengthChecker) addAdvice(tableName, columnName string, line int) {
	c.adviceList = append(c.adviceList, &types.Advice{
		Status: c.level,
		Code:   int32(types.CharLengthExceedsLimit),
		Title:  c.title,
		Content: fmt.Sprintf(
			"The length of the CHAR column %q in table %s is bigger than %d, please use VARCHAR instead",
			columnName,
			tableName,
			c.maximum,
		),
		StartPosition: &types.Position{
			Line: int32(line),
		},
	})
}
