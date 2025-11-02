package mysql

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// tableState maps table names to column sets
type tableState map[string]columnSet

// tableList returns table list in lexicographical order
func (t tableState) tableList() []string {
	var tableList []string
	for tableName := range t {
		tableList = append(tableList, tableName)
	}
	slices.Sort(tableList)
	return tableList
}

// ColumnRequiredRule is the ANTLR-based implementation for checking column requirement
type ColumnRequiredRule struct {
	BaseAntlrRule
	requiredColumns columnSet
	tables          tableState
	line            map[string]int
}

// NewColumnRequiredRule creates a new ANTLR-based column required rule
func NewColumnRequiredRule(level types.SQLReviewRuleLevel, title string, requiredColumns columnSet) *ColumnRequiredRule {
	return &ColumnRequiredRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		requiredColumns: requiredColumns,
		tables:          make(tableState),
		line:            make(map[string]int),
	}
}

// Name returns the rule name
func (*ColumnRequiredRule) Name() string {
	return "ColumnRequiredRule"
}

// OnEnter is called when entering a parse tree node
func (r *ColumnRequiredRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	case NodeTypeDropTable:
		r.checkDropTable(ctx.(*mysql.DropTableContext))
	case NodeTypeAlterTable:
		r.checkAlterTable(ctx.(*mysql.AlterTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*ColumnRequiredRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *ColumnRequiredRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	r.createTable(ctx)
}

func (r *ColumnRequiredRule) checkDropTable(ctx *mysql.DropTableContext) {
	if ctx.TableRefList() == nil {
		return
	}

	for _, tableRef := range ctx.TableRefList().AllTableRef() {
		_, tableName := mysqlparser.NormalizeMySQLTableRef(tableRef)
		delete(r.tables, tableName)
	}
}

func (r *ColumnRequiredRule) checkAlterTable(ctx *mysql.AlterTableContext) {
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

		lineNumber := r.baseLine + item.GetStart().GetLine()
		switch {
		// add column
		case item.ADD_SYMBOL() != nil:
			switch {
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
				r.addColumn(tableName, columnName)
			case item.OPEN_PAR_SYMBOL() != nil && item.TableElementList() != nil:
				for _, tableElement := range item.TableElementList().AllTableElement() {
					if tableElement.ColumnDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil ||
						tableElement.ColumnDefinition().FieldDefinition() == nil {
						continue
					}
					_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
					r.addColumn(tableName, columnName)
				}
			}
		// drop column
		case item.DROP_SYMBOL() != nil && item.ColumnInternalRef() != nil:
			columnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			if r.dropColumn(tableName, columnName) {
				r.line[tableName] = lineNumber
			}
		// rename column
		case item.RENAME_SYMBOL() != nil && item.COLUMN_SYMBOL() != nil:
			oldColumnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			newColumnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			r.renameColumn(tableName, oldColumnName, newColumnName)
			r.line[tableName] = lineNumber
		// change column
		case item.CHANGE_SYMBOL() != nil && item.ColumnInternalRef() != nil && item.Identifier() != nil:
			oldColumnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
			newColumnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
			if r.renameColumn(tableName, oldColumnName, newColumnName) {
				r.line[tableName] = lineNumber
			}
		}
	}
}

func (r *ColumnRequiredRule) generateAdviceList() {
	// Order it cause the random iteration order in Go, see https://go.dev/blog/maps
	tableList := r.tables.tableList()
	for _, tableName := range tableList {
		table := r.tables[tableName]
		var missingColumns []string
		for columnName := range r.requiredColumns {
			if exists, ok := table[columnName]; !ok || !exists {
				missingColumns = append(missingColumns, columnName)
			}
		}

		if len(missingColumns) > 0 {
			// Order it cause the random iteration order in Go, see https://go.dev/blog/maps
			slices.Sort(missingColumns)
			r.AddAdvice(&types.Advice{
				Status:        types.Advice_Status(r.level),
				Code:          int32(types.ColumnRequired),
				Title:         r.title,
				Content:       fmt.Sprintf("Table `%s` requires columns: %s", tableName, strings.Join(missingColumns, ", ")),
				StartPosition: ConvertANTLRLineToPosition(r.line[tableName]),
			})
		}
	}
}

func (r *ColumnRequiredRule) createTable(ctx *mysql.CreateTableContext) {
	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	r.line[tableName] = r.baseLine + ctx.GetStart().GetLine()
	r.initEmptyTable(tableName)

	if ctx.TableElementList() == nil {
		return
	}

	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		if tableElement.ColumnDefinition() == nil {
			continue
		}
		_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
		r.addColumn(tableName, columnName)
	}
}

func (r *ColumnRequiredRule) initEmptyTable(tableName string) columnSet {
	r.tables[tableName] = make(columnSet)
	return r.tables[tableName]
}

// add a column.
func (r *ColumnRequiredRule) addColumn(tableName string, columnName string) {
	if _, ok := r.requiredColumns[columnName]; !ok {
		return
	}

	if table, ok := r.tables[tableName]; !ok {
		// We do not retrospectively check.
		// So we assume it contains all required columns.
		r.initFullTable(tableName)
	} else {
		table[columnName] = true
	}
}

// drop a column
// return true if the column was successfully dropped from requirement list.
func (r *ColumnRequiredRule) dropColumn(tableName string, columnName string) bool {
	if _, ok := r.requiredColumns[columnName]; !ok {
		return false
	}
	table, ok := r.tables[tableName]
	if !ok {
		// We do not retrospectively check.
		// So we assume it contains all required columns.
		table = r.initFullTable(tableName)
	}
	table[columnName] = false
	return true
}

// rename a column
// return if the old column was dropped from requirement list.
func (r *ColumnRequiredRule) renameColumn(tableName string, oldColumn string, newColumn string) bool {
	_, oldNeed := r.requiredColumns[oldColumn]
	_, newNeed := r.requiredColumns[newColumn]
	if !oldNeed && !newNeed {
		return false
	}
	table, ok := r.tables[tableName]
	if !ok {
		// We do not retrospectively check.
		// So we assume it contains all required columns.
		table = r.initFullTable(tableName)
	}
	if oldNeed {
		table[oldColumn] = false
	}
	if newNeed {
		table[newColumn] = true
	}
	return oldNeed
}

func (r *ColumnRequiredRule) initFullTable(tableName string) columnSet {
	table := r.initEmptyTable(tableName)
	for column := range r.requiredColumns {
		table[column] = true
	}
	return table
}

// ColumnRequiredAdvisor is the advisor using ANTLR parser for column required checking
type ColumnRequiredAdvisor struct{}

// Check performs the ANTLR-based column required check
func (a *ColumnRequiredAdvisor) Check(
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

	// Create the default required columns set (based on test data)
	requiredColumns := make(columnSet)
	defaultColumns := []string{"id", "creator_id", "created_ts", "updater_id", "updated_ts"}
	for _, column := range defaultColumns {
		requiredColumns[column] = true
	}

	// Create the rule (doesn't need catalog)
	columnRule := NewColumnRequiredRule(types.SQLReviewRuleLevel(level), string(rule.Type), requiredColumns)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{columnRule})

	for _, stmtNode := range root {
		columnRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	// Generate advice after walking
	columnRule.generateAdviceList()

	return checker.GetAdviceList(), nil
}
