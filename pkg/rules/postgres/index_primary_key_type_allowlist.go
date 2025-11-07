package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*IndexPrimaryKeyTypeAllowlistAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist),
		&IndexPrimaryKeyTypeAllowlistAdvisor{},
	)
}

type IndexPrimaryKeyTypeAllowlistAdvisor struct{}

func (*IndexPrimaryKeyTypeAllowlistAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &indexPrimaryKeyTypeAllowlistChecker{
		level:     level,
		title:     string(checkCtx.Rule.Type),
		allowlist: payload.List,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type indexPrimaryKeyTypeAllowlistChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	allowlist  []string
}

func (c *indexPrimaryKeyTypeAllowlistChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	columnTypes := make(map[string]string)
	columnLines := make(map[string]int)

	if ctx.Opttableelementlist() != nil && ctx.Opttableelementlist().Tableelementlist() != nil {
		allElements := ctx.Opttableelementlist().Tableelementlist().AllTableelement()
		for _, elem := range allElements {
			if elem.ColumnDef() != nil {
				colDef := elem.ColumnDef()
				if colDef.Colid() != nil && colDef.Typename() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(colDef.Colid())
					columnType := c.getTypeName(colDef.Typename())
					columnTypes[columnName] = columnType
					columnLines[columnName] = colDef.GetStart().GetLine()

					if c.hasColumnPrimaryKeyConstraint(colDef) {
						if !c.isTypeAllowed(columnType) {
							c.addAdvice(columnName, columnType, colDef.GetStart().GetLine())
						}
					}
				}
			}
		}

		for _, elem := range allElements {
			if elem.Tableconstraint() != nil {
				c.checkTablePrimaryKey(elem.Tableconstraint(), columnTypes, columnLines)
			}
		}
	}
}

func (*indexPrimaryKeyTypeAllowlistChecker) hasColumnPrimaryKeyConstraint(colDef parser.IColumnDefContext) bool {
	if colDef.Colquallist() == nil {
		return false
	}

	allConstraints := colDef.Colquallist().AllColconstraint()
	for _, constraint := range allConstraints {
		if constraint.Colconstraintelem() != nil {
			elem := constraint.Colconstraintelem()
			if elem.PRIMARY() != nil && elem.KEY() != nil {
				return true
			}
		}
	}

	return false
}

func (c *indexPrimaryKeyTypeAllowlistChecker) checkTablePrimaryKey(
	constraint parser.ITableconstraintContext,
	columnTypes map[string]string,
	columnLines map[string]int,
) {
	if constraint == nil || constraint.Constraintelem() == nil {
		return
	}

	elem := constraint.Constraintelem()

	if elem.PRIMARY() != nil && elem.KEY() != nil {
		if elem.Columnlist() != nil {
			allColumns := elem.Columnlist().AllColumnElem()
			for _, col := range allColumns {
				if col.Colid() != nil {
					columnName := pgparser.NormalizePostgreSQLColid(col.Colid())
					if columnType, exists := columnTypes[columnName]; exists {
						if !c.isTypeAllowed(columnType) {
							line := columnLines[columnName]
							c.addAdvice(columnName, columnType, line)
						}
					}
				}
			}
		}
	}
}

func (*indexPrimaryKeyTypeAllowlistChecker) getTypeName(typename parser.ITypenameContext) string {
	if typename == nil {
		return ""
	}

	return normalizePostgreSQLType(typename.GetText())
}

func (c *indexPrimaryKeyTypeAllowlistChecker) isTypeAllowed(columnType string) bool {
	return isTypeInList(columnType, c.allowlist)
}

func (c *indexPrimaryKeyTypeAllowlistChecker) addAdvice(columnName, columnType string, line int) {
	c.adviceList = append(c.adviceList, &types.Advice{
		Status: c.level,
		Code:   int32(advisor.PostgreSQLPrimaryKeyTypeAllowlist),
		Title:  c.title,
		Content: fmt.Sprintf(
			"The column %q is one of the primary key, but its type %q is not in allowlist",
			columnName,
			columnType,
		),
		StartPosition: &types.Position{
			Line: int32(line),
		},
	})
}
