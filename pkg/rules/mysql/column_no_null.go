package mysql

import (
	"context"
	"fmt"
	"slices"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/pkg/errors"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type columnName struct {
	tableName  string
	columnName string
	line       int
}

func (c columnName) name() string {
	return fmt.Sprintf("%s.%s", c.tableName, c.columnName)
}

// ColumnNoNullRule is the ANTLR-based implementation for checking non-null column requirement
type ColumnNoNullRule struct {
	BaseAntlrRule
	columnSet map[string]columnName
	catalog   *catalog.Finder
}

// NewColumnNoNullRule creates a new ANTLR-based non-null column rule
func NewColumnNoNullRule(level types.SQLReviewRuleLevel, title string, catalog *catalog.Finder) *ColumnNoNullRule {
	return &ColumnNoNullRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		columnSet: make(map[string]columnName),
		catalog:   catalog,
	}
}

// Name returns the rule name
func (*ColumnNoNullRule) Name() string {
	return "ColumnNoNullRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnNoNullRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnNoNullRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnNoNullRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableName() == nil {
		return
	}
	if ctx.TableElementList() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement == nil {
			continue
		}
		if tableElement.ColumnDefinition() == nil {
			continue
		}

		_, _, column := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		fieldDef := tableElement.ColumnDefinition().FieldDefinition()
		if fieldDef == nil {
			continue
		}
		
		// Check if column is nullable (no NOT NULL constraint and not PRIMARY KEY)
		if r.isColumnNullable(fieldDef) {
			col := columnName{
				tableName:  tableName,
				columnName: column,
				line:       r.baseLine + tableElement.GetStart().GetLine(),
			}
			if _, exists := r.columnSet[col.name()]; !exists {
				r.columnSet[col.name()] = col
			}
		}
	}
}

func (r *ColumnNoNullRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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
	// alter table add column, change column, modify column.
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		switch {
		// ADD COLUMN single column
		case item.ADD_SYMBOL() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			column := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			if r.isColumnNullable(item.FieldDefinition()) {
				col := columnName{
					tableName:  tableName,
					columnName: column,
					line:       r.baseLine + item.GetStart().GetLine(),
				}
				if _, exists := r.columnSet[col.name()]; !exists {
					r.columnSet[col.name()] = col
				}
			}
		// ADD COLUMN multiple columns
		case item.ADD_SYMBOL() != nil && item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
			for _, tableElement := range item.TableElementList().AllTableElement() {
				if tableElement.ColumnDefinition() == nil {
					continue
				}
				_, _, column := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
				fieldDef := tableElement.ColumnDefinition().FieldDefinition()
				if fieldDef != nil && r.isColumnNullable(fieldDef) {
					col := columnName{
						tableName:  tableName,
						columnName: column,
						line:       r.baseLine + item.GetStart().GetLine(),
					}
					if _, exists := r.columnSet[col.name()]; !exists {
						r.columnSet[col.name()] = col
					}
				}
			}
		// CHANGE COLUMN
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil && item.FieldDefinition() != nil:
			column := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			if r.isColumnNullable(item.FieldDefinition()) {
				col := columnName{
					tableName:  tableName,
					columnName: column,
					line:       r.baseLine + item.GetStart().GetLine(),
				}
				if _, exists := r.columnSet[col.name()]; !exists {
					r.columnSet[col.name()] = col
				}
			}
		}
	}
}

// isColumnNullable determines if a column is nullable based on its field definition
func (r *ColumnNoNullRule) isColumnNullable(fieldDef mysql.IFieldDefinitionContext) bool {
	if fieldDef == nil {
		return true // Default to nullable if no definition
	}

	// Check for explicit NOT NULL constraint or PRIMARY KEY constraint
	for _, attr := range fieldDef.AllColumnAttribute() {
		if attr == nil {
			continue
		}

		// Check for NOT NULL
		if attr.NullLiteral() != nil && attr.NOT_SYMBOL() != nil {
			return false
		}

		// Check for PRIMARY KEY constraint (implies NOT NULL)
		if attr.PRIMARY_SYMBOL() != nil && attr.KEY_SYMBOL() != nil {
			return false
		}
	}

	return true // Default to nullable
}

func (r *ColumnNoNullRule) generateAdvice() {
	var columnList []columnName
	for _, column := range r.columnSet {
		columnList = append(columnList, column)
	}
	slices.SortFunc(columnList, func(a, b columnName) int {
		if a.line != b.line {
			if a.line < b.line {
				return -1
			}
			return 1
		}
		if a.columnName < b.columnName {
			return -1
		}
		if a.columnName > b.columnName {
			return 1
		}
		return 0
	})

	for _, column := range columnList {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.ColumnCannotNull),
			Title:         r.title,
			Content:       fmt.Sprintf("`%s`.`%s` cannot have NULL value", column.tableName, column.columnName),
			StartPosition: ConvertANTLRLineToPosition(column.line),
		})
	}
}

// ColumnNoNullAdvisor is the advisor using ANTLR parser for non-null column checking
type ColumnNoNullAdvisor struct{}

// Check performs the ANTLR-based non-null column check
func (a *ColumnNoNullAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Create the rule without catalog (not needed for static analysis)
	columnRule := NewColumnNoNullRule(types.SQLReviewRuleLevel(level), string(rule.Type), nil)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{columnRule})

	// Apply the rule to analyze column definitions
	for _, stmtNode := range root {
		columnRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	// Generate advice after walking
	columnRule.generateAdvice()

	return checker.GetAdviceList(), nil
}
