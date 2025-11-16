package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*SchemaBackwardCompatibilityAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleSchemaBackwardCompatibility),
		&SchemaBackwardCompatibilityAdvisor{},
	)
}

// SchemaBackwardCompatibilityAdvisor checks for schema backward compatibility.
type SchemaBackwardCompatibilityAdvisor struct{}

// Check checks the schema backward compatibility.
func (*SchemaBackwardCompatibilityAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &schemaBackwardCompatibilityChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		statementsText:               checkCtx.Statements,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type schemaBackwardCompatibilityChecker struct {
	*parser.BasePostgreSQLParserListener

	level           types.Advice_Status
	title           string
	lastCreateTable string
	adviceList      []*types.Advice
	statementsText  string
}

// EnterCreatestmt tracks CREATE TABLE statements
func (c *schemaBackwardCompatibilityChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	qualifiedNames := ctx.AllQualified_name()
	if len(qualifiedNames) > 0 {
		c.lastCreateTable = extractTableName(qualifiedNames[0])
	}
}

// EnterDropdbstmt handles DROP DATABASE
func (c *schemaBackwardCompatibilityChecker) EnterDropdbstmt(ctx *parser.DropdbstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
	c.adviceList = append(c.adviceList, &types.Advice{
		Status:  c.level,
		Code:    int32(types.CompatibilityDropDatabase),
		Title:   c.title,
		Content: fmt.Sprintf(`"%s" may cause incompatibility with the existing data and code`, stmtText),
		StartPosition: &types.Position{
			Line: int32(ctx.GetStart().GetLine()),
		},
	})
}

// EnterDropstmt handles DROP TABLE/VIEW
func (c *schemaBackwardCompatibilityChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is DROP TABLE or DROP VIEW
	if ctx.Object_type_any_name() != nil {
		objType := ctx.Object_type_any_name()
		if objType.TABLE() != nil || objType.VIEW() != nil {
			stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(types.CompatibilityDropTable),
				Title:   c.title,
				Content: fmt.Sprintf(`"%s" may cause incompatibility with the existing data and code`, stmtText),
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
		}
	}
}

// EnterRenamestmt handles ALTER TABLE RENAME and RENAME COLUMN
func (c *schemaBackwardCompatibilityChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	code := types.CompatibilityOK

	// Check if this is a column rename
	if ctx.Opt_column() != nil && ctx.Opt_column().COLUMN() != nil {
		// RENAME COLUMN - check if not on last created table
		if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
			tableName := extractTableName(ctx.Relation_expr().Qualified_name())
			if c.lastCreateTable != tableName {
				code = types.CompatibilityRenameColumn
			}
		}
	} else {
		// RENAME TABLE/VIEW
		code = types.CompatibilityRenameTable
	}

	if code != types.CompatibilityOK {
		stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(code),
			Title:   c.title,
			Content: fmt.Sprintf(`"%s" may cause incompatibility with the existing data and code`, stmtText),
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}

// EnterAltertablestmt handles various ALTER TABLE commands
func (c *schemaBackwardCompatibilityChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}
	tableName := extractTableName(ctx.Relation_expr().Qualified_name())

	// Skip if this is the table we just created
	if c.lastCreateTable == tableName {
		return
	}

	if ctx.Alter_table_cmds() == nil {
		return
	}

	allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
	for _, cmd := range allCmds {
		code := types.CompatibilityOK

		// DROP COLUMN
		if cmd.DROP() != nil && cmd.Opt_column() != nil && cmd.Opt_column().COLUMN() != nil {
			code = types.CompatibilityDropColumn
		}

		// ALTER COLUMN TYPE
		if cmd.ALTER() != nil && cmd.TYPE_P() != nil {
			code = types.CompatibilityAlterColumn
		}

		// ADD CONSTRAINT
		if cmd.ADD_P() != nil && cmd.Tableconstraint() != nil {
			constraint := cmd.Tableconstraint()
			if constraint.Constraintelem() != nil {
				elem := constraint.Constraintelem()

				// PRIMARY KEY
				if elem.PRIMARY() != nil && elem.KEY() != nil {
					code = types.CompatibilityAddPrimaryKey
				}

				// UNIQUE
				if elem.UNIQUE() != nil {
					code = types.CompatibilityAddUniqueKey
				}

				// FOREIGN KEY
				if elem.FOREIGN() != nil && elem.KEY() != nil {
					code = types.CompatibilityAddForeignKey
				}

				// CHECK - only if NOT VALID is not present
				if elem.CHECK() != nil {
					// Check if NOT VALID is present in constraint attributes
					hasNotValid := false
					if elem.Constraintattributespec() != nil {
						allAttrs := elem.Constraintattributespec().AllConstraintattributeElem()
						for _, attr := range allAttrs {
							if attr.NOT() != nil && attr.VALID() != nil {
								hasNotValid = true
								break
							}
						}
					}
					if !hasNotValid {
						code = types.CompatibilityAddCheck
					}
				}
			}
		}

		if code != types.CompatibilityOK {
			stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(code),
				Title:   c.title,
				Content: fmt.Sprintf(`"%s" may cause incompatibility with the existing data and code`, stmtText),
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
			return
		}
	}
}

// EnterIndexstmt handles CREATE UNIQUE INDEX
func (c *schemaBackwardCompatibilityChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is CREATE UNIQUE INDEX
	if ctx.Opt_unique() == nil || ctx.Opt_unique().UNIQUE() == nil {
		return
	}

	// Get table name
	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}
	tableName := extractTableName(ctx.Relation_expr().Qualified_name())

	// Skip if this is the table we just created
	if c.lastCreateTable == tableName {
		return
	}

	stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
	c.adviceList = append(c.adviceList, &types.Advice{
		Status:  c.level,
		Code:    int32(types.CompatibilityAddUniqueKey),
		Title:   c.title,
		Content: fmt.Sprintf(`"%s" may cause incompatibility with the existing data and code`, stmtText),
		StartPosition: &types.Position{
			Line: int32(ctx.GetStart().GetLine()),
		},
	})
}
