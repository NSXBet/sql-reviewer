package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

type SchemaBackwardCompatibilityAdvisor struct{}

func (a *SchemaBackwardCompatibilityAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	stmtList, errAdvice := mysqlparser.ParseMySQL(statements)
	if errAdvice != nil {
		return ConvertSyntaxErrorToAdvice(errAdvice)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule
	compatibilityRule := NewSchemaBackwardCompatibilityRule(level, string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericChecker([]Rule{compatibilityRule})

	for _, stmt := range stmtList {
		compatibilityRule.SetBaseLine(stmt.BaseLine)
		checker.SetBaseLine(stmt.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmt.Tree)
	}

	return checker.GetAdviceList(), nil
}

// SchemaBackwardCompatibilityRule checks for schema backward compatibility.
type SchemaBackwardCompatibilityRule struct {
	BaseRule
	text            string
	lastCreateTable string
	code            int32
}

// NewSchemaBackwardCompatibilityRule creates a new SchemaBackwardCompatibilityRule.
func NewSchemaBackwardCompatibilityRule(level types.Advice_Status, title string) *SchemaBackwardCompatibilityRule {
	return &SchemaBackwardCompatibilityRule{
		BaseRule: BaseRule{
			level: level,
			title: title,
		},
		code: 0, // OK
	}
}

// Name returns the rule name.
func (*SchemaBackwardCompatibilityRule) Name() string {
	return "SchemaBackwardCompatibilityRule"
}

// OnEnter is called when entering a parse tree node.
func (r *SchemaBackwardCompatibilityRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeQuery:
		queryCtx, ok := ctx.(*mysql.QueryContext)
		if !ok {
			return nil
		}
		r.text = queryCtx.GetParser().GetTokenStream().GetTextFromRuleContext(queryCtx)
		r.code = 0 // OK
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeDropDatabase:
		r.checkDropDatabase(ctx.(*mysql.DropDatabaseContext))
	case NodeTypeRenameTableStatement:
		r.checkRenameTableStatement(ctx.(*mysql.RenameTableStatementContext))
	case NodeTypeDropTable:
		r.checkDropTable(ctx.(*mysql.DropTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	case NodeTypeCreateIndex:
		r.checkCreateIndex(ctx.(*mysql.CreateIndexContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node.
func (r *SchemaBackwardCompatibilityRule) OnExit(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeQuery && r.code != 0 {
		r.AddAdvice(&types.Advice{
			Status:        r.level,
			Code:          r.code,
			Title:         r.title,
			Content:       fmt.Sprintf("\"%s\" may cause incompatibility with the existing data and code", r.text),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
	return nil
}

func (r *SchemaBackwardCompatibilityRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	r.lastCreateTable = tableName
}

func (r *SchemaBackwardCompatibilityRule) checkDropDatabase(ctx *mysql.DropDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	r.code = int32(types.CompatibilityDropDatabase)
}

func (r *SchemaBackwardCompatibilityRule) checkRenameTableStatement(ctx *mysql.RenameTableStatementContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	r.code = int32(types.CompatibilityRenameTable)
}

func (r *SchemaBackwardCompatibilityRule) checkDropTable(ctx *mysql.DropTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	r.code = int32(types.CompatibilityDropTable)
}

func (r *SchemaBackwardCompatibilityRule) checkAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.AlterTableActions() == nil || ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	if ctx.TableRef() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	if tableName == r.lastCreateTable {
		return
	}

	// alter table add column, change column, modify column.
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		if item.RENAME_SYMBOL() != nil && item.COLUMN_SYMBOL() != nil {
			r.code = int32(types.CompatibilityRenameColumn)
			return
		}

		if item.DROP_SYMBOL() != nil && item.ColumnInternalRef() != nil {
			r.code = int32(types.CompatibilityDropColumn)
			return
		}
		if item.DROP_SYMBOL() != nil && item.TableName() != nil {
			r.code = int32(types.CompatibilityRenameTable)
			return
		}

		if item.ADD_SYMBOL() != nil {
			if item.TableConstraintDef() != nil {
				if item.TableConstraintDef().GetType_() == nil {
					continue
				}
				switch item.TableConstraintDef().GetType_().GetTokenType() {
				// add primary key.
				case mysql.MySQLParserPRIMARY_SYMBOL:
					r.code = int32(types.CompatibilityAddPrimaryKey)
					return
				// add unique key.
				case mysql.MySQLParserUNIQUE_SYMBOL:
					r.code = int32(types.CompatibilityAddUniqueKey)
					return
				// add foreign key.
				case mysql.MySQLParserFOREIGN_SYMBOL:
					r.code = int32(types.CompatibilityAddForeignKey)
					return
				}
			}

			// add check enforced.
			// Check is only supported after 8.0.16 https://dev.mysql.com/doc/refman/8.0/en/create-table-check-constraints.html
			if item.TableConstraintDef() != nil && item.TableConstraintDef().CheckConstraint() != nil &&
				item.TableConstraintDef().ConstraintEnforcement() != nil {
				r.code = int32(types.CompatibilityAddCheck)
				return
			}
		}

		// add check enforced.
		// Check is only supported after 8.0.16 https://dev.mysql.com/doc/refman/8.0/en/create-table-check-constraints.html
		if item.ALTER_SYMBOL() != nil && item.CHECK_SYMBOL() != nil {
			if item.ConstraintEnforcement() != nil {
				r.code = int32(types.CompatibilityAlterCheck)
				return
			}
		}

		// MODIFY COLUMN / CHANGE COLUMN
		// Due to the limitation that we don't know the current data type of the column before the change,
		// so we treat all as incompatible. This generates false positive when:
		// 1. Change to a compatible data type such as INT to BIGINT
		// 2. Change properties such as comment, change it to NULL
		// modify column
		if item.MODIFY_SYMBOL() != nil && item.ColumnInternalRef() != nil {
			r.code = int32(types.CompatibilityAlterColumn)
			return
		}
		// change column
		if item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil {
			r.code = int32(types.CompatibilityAlterColumn)
			return
		}
	}
}

func (r *SchemaBackwardCompatibilityRule) checkCreateIndex(ctx *mysql.CreateIndexContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.GetType_() == nil {
		return
	}
	if ctx.GetType_().GetTokenType() != mysql.MySQLParserUNIQUE_SYMBOL {
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.CreateIndexTarget().TableRef())
	if r.lastCreateTable != tableName {
		r.code = int32(types.CompatibilityAddUniqueKey)
	}
}
