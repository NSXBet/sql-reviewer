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
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleSchemaBackwardCompatibility), &SchemaBackwardCompatibilityAdvisor{})
}

// SchemaBackwardCompatibilityAdvisor checks for schema backward compatibility.
type SchemaBackwardCompatibilityAdvisor struct{}

// Check checks the schema backward compatibility.
func (*SchemaBackwardCompatibilityAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return nil, err
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	checker := &schemaBackwardCompatibilityChecker{
		level: level,
		title: string(checkCtx.Rule.Type),
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type schemaBackwardCompatibilityChecker struct {
	*parser.BasePostgreSQLParserListener

	level            types.Advice_Status
	title            string
	lastCreateTable  string
	adviceList       []*types.Advice
	currentStmtStart int
	currentStmtText  string
}

// EnterStmt captures the current statement
func (c *schemaBackwardCompatibilityChecker) EnterStmt(ctx *parser.StmtContext) {
	c.currentStmtStart = ctx.GetStart().GetLine()
	c.currentStmtText = ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
}

// EnterCreatestmt tracks CREATE TABLE statements
func (c *schemaBackwardCompatibilityChecker) EnterCreatestmt(ctx *parser.CreatestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if len(ctx.AllQualified_name()) > 0 {
		tableName := extractTableName(ctx.AllQualified_name()[0])
		c.lastCreateTable = tableName
	}
}

// EnterDropdbstmt checks DROP DATABASE/SCHEMA
func (c *schemaBackwardCompatibilityChecker) EnterDropdbstmt(ctx *parser.DropdbstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// DROP DATABASE or DROP SCHEMA both use DropdbstmtContext
	c.addIncompatibilityAdvice(ctx.GetStart().GetLine(), types.CompatibilityDropDatabase)
}

// EnterDropstmt checks DROP TABLE
func (c *schemaBackwardCompatibilityChecker) EnterDropstmt(ctx *parser.DropstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is DROP TABLE by checking if Object_type_any_name has TABLE
	if ctx.Object_type_any_name() != nil && ctx.Object_type_any_name().TABLE() != nil {
		c.addIncompatibilityAdvice(ctx.GetStart().GetLine(), types.CompatibilityDropTable)
	}
}

// EnterRenamestmt checks table rename
func (c *schemaBackwardCompatibilityChecker) EnterRenamestmt(ctx *parser.RenamestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// RENAME TABLE - PostgreSQL uses ALTER TABLE RENAME syntax, so this might not catch all cases
	// Most table renames are handled in EnterAltertablestmt
	c.addIncompatibilityAdvice(ctx.GetStart().GetLine(), types.CompatibilityRenameTable)
}

// EnterAltertablestmt checks ALTER TABLE for incompatible changes
func (c *schemaBackwardCompatibilityChecker) EnterAltertablestmt(ctx *parser.AltertablestmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	if ctx.Relation_expr() == nil || ctx.Relation_expr().Qualified_name() == nil {
		return
	}

	tableName := extractTableName(ctx.Relation_expr().Qualified_name())

	// Skip if this is the table we just created in this script
	if tableName == c.lastCreateTable {
		return
	}

	// Check ALTER TABLE commands
	if ctx.Alter_table_cmds() != nil {
		allCmds := ctx.Alter_table_cmds().AllAlter_table_cmd()
		for _, cmd := range allCmds {
			// Check for specific ALTER TABLE command types by inspecting the context
			cmdText := cmd.GetText()

			// RENAME COLUMN: ALTER TABLE ... RENAME COLUMN ... TO ...
			if cmd.GetChild(0) != nil && cmd.GetChild(0).(antlr.ParseTree).GetText() == "RENAME" {
				// Check if next token is COLUMN
				if cmd.GetChildCount() > 1 {
					secondToken := cmd.GetChild(1).(antlr.ParseTree).GetText()
					if secondToken == "COLUMN" {
						c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityRenameColumn)
						return
					}
					// RENAME TO (table rename)
					if secondToken == "TO" {
						c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityRenameTable)
						return
					}
				}
			}

			// DROP COLUMN
			if cmd.GetChild(0) != nil && cmd.GetChild(0).(antlr.ParseTree).GetText() == "DROP" {
				if cmd.GetChildCount() > 1 && cmd.GetChild(1).(antlr.ParseTree).GetText() == "COLUMN" {
					c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityDropColumn)
					return
				}
			}

			// ADD constraint types
			if cmd.GetChild(0) != nil && cmd.GetChild(0).(antlr.ParseTree).GetText() == "ADD" {
				// Check for ADD PRIMARY KEY, ADD UNIQUE, ADD FOREIGN KEY, ADD CHECK
				if cmd.Tableconstraint() != nil && cmd.Tableconstraint().Constraintelem() != nil {
					elem := cmd.Tableconstraint().Constraintelem()
					elemText := elem.GetText()

					if elem.PRIMARY() != nil {
						c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityAddPrimaryKey)
						return
					}
					if elem.UNIQUE() != nil {
						c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityAddUniqueKey)
						return
					}
					if elem.FOREIGN() != nil {
						c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityAddForeignKey)
						return
					}
					if elem.CHECK() != nil {
						c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityAddCheck)
						return
					}
					_ = elemText
				}
			}

			// ALTER COLUMN TYPE (column type change)
			if cmd.GetChild(0) != nil && cmd.GetChild(0).(antlr.ParseTree).GetText() == "ALTER" {
				// Check if this is ALTER COLUMN ... TYPE
				if cmd.Typename() != nil {
					c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityAlterColumn)
					return
				}
			}

			// VALIDATE CONSTRAINT
			if cmd.GetChild(0) != nil && cmd.GetChild(0).(antlr.ParseTree).GetText() == "VALIDATE" {
				if cmdText != "" && len(cmdText) > 8 && cmdText[:8] == "VALIDATE" {
					c.addIncompatibilityAdvice(cmd.GetStart().GetLine(), types.CompatibilityAlterCheck)
					return
				}
			}
		}
	}
}

// EnterIndexstmt checks CREATE UNIQUE INDEX on existing table
func (c *schemaBackwardCompatibilityChecker) EnterIndexstmt(ctx *parser.IndexstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Check if this is CREATE UNIQUE INDEX by looking at the children
	hasUnique := false
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if child != nil {
			childText := child.(antlr.ParseTree).GetText()
			if childText == "UNIQUE" {
				hasUnique = true
				break
			}
		}
	}

	if hasUnique {
		if ctx.Relation_expr() != nil && ctx.Relation_expr().Qualified_name() != nil {
			tableName := extractTableName(ctx.Relation_expr().Qualified_name())
			// Only flag if it's not on the table we just created
			if tableName != c.lastCreateTable {
				c.addIncompatibilityAdvice(ctx.GetStart().GetLine(), types.CompatibilityAddUniqueKey)
			}
		}
	}
}

func (c *schemaBackwardCompatibilityChecker) addIncompatibilityAdvice(line int, compatibilityCode int) {
	c.adviceList = append(c.adviceList, &types.Advice{
		Status:  c.level,
		Code:    int32(compatibilityCode),
		Title:   c.title,
		Content: fmt.Sprintf("\"%s\" may cause incompatibility with the existing data and code", c.currentStmtText),
		StartPosition: &types.Position{
			Line: int32(line),
		},
	})
}
