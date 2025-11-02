package mysql

import (
	"log/slog"
	"reflect"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// Node type constants for consistent node type checking
const (
	NodeTypeCreateTable             = "CreateTable"
	NodeTypeAlterTable              = "AlterTable"
	NodeTypeAlterStatement          = "AlterStatement"
	NodeTypeDropTable               = "DropTable"
	NodeTypeRenameTableStatement    = "RenameTableStatement"
	NodeTypeSetStatement            = "SetStatement"
	NodeTypeCreateIndex             = "CreateIndex"
	NodeTypeDropIndex               = "DropIndex"
	NodeTypeInsertStatement         = "InsertStatement"
	NodeTypeUpdateStatement         = "UpdateStatement"
	NodeTypeDeleteStatement         = "DeleteStatement"
	NodeTypeSelectStatement         = "SelectStatement"
	NodeTypeCreateView              = "CreateView"
	NodeTypeDropView                = "DropView"
	NodeTypeCreateProcedure         = "CreateProcedure"
	NodeTypeDropProcedure           = "DropProcedure"
	NodeTypeCreateFunction          = "CreateFunction"
	NodeTypeDropFunction            = "DropFunction"
	NodeTypeCreateEvent             = "CreateEvent"
	NodeTypeDropEvent               = "DropEvent"
	NodeTypeCreateTrigger           = "CreateTrigger"
	NodeTypeDropTrigger             = "DropTrigger"
	NodeTypeQuery                   = "Query"
	NodeTypeQueryExpression         = "QueryExpression"
	NodeTypeFunctionCall            = "FunctionCall"
	NodeTypeCreateDatabase          = "CreateDatabase"
	NodeTypeAlterDatabase           = "AlterDatabase"
	NodeTypeDropDatabase            = "DropDatabase"
	NodeTypeTruncateTableStatement  = "TruncateTableStatement"
	NodeTypeSelectStatementWithInto = "SelectStatementWithInto"
	NodeTypeSelectItemList          = "SelectItemList"
	NodeTypePureIdentifier          = "PureIdentifier"
	NodeTypeIdentifierKeyword       = "IdentifierKeyword"
	NodeTypeTransactionStatement    = "TransactionStatement"
	NodeTypePredicateExprLike       = "PredicateExprLike"
	NodeTypeSimpleStatement         = "SimpleStatement"
	NodeTypeQuerySpecification      = "QuerySpecification"
	NodeTypeLimitClause             = "LimitClause"
	NodeTypeFromClause              = "FromClause"
	NodeTypePrimaryExprCompare      = "PrimaryExprCompare"
	NodeTypeJoinedTable             = "JoinedTable"
	NodeTypeWhereClause             = "WhereClause"
	NodeTypePredicateExprIn         = "PredicateExprIn"
	NodeTypeExprList                = "ExprList"
	NodeTypeExprOr                  = "ExprOr"
	NodeTypeAlterTableActions       = "AlterTableActions"
)

// Rule defines the interface for individual SQL validation rules.
// Each rule implements specific checking logic without embedding the base listener.
type Rule interface {
	// OnEnter is called when entering a parse tree node
	OnEnter(ctx antlr.ParserRuleContext, nodeType string) error

	// OnExit is called when exiting a parse tree node
	OnExit(ctx antlr.ParserRuleContext, nodeType string) error

	// Name returns the rule name for logging/debugging
	Name() string

	// GetAdviceList returns the accumulated advice from this rule
	GetAdviceList() []*types.Advice
}

// AntlrRule defines the interface for individual SQL validation rules using ANTLR.
// Each rule implements specific checking logic without embedding the base listener.
type AntlrRule interface {
	// OnEnter is called when entering a parse tree node
	OnEnter(ctx antlr.ParserRuleContext, nodeType string) error

	// OnExit is called when exiting a parse tree node
	OnExit(ctx antlr.ParserRuleContext, nodeType string) error

	// Name returns the rule name for logging/debugging
	Name() string

	// GetAdviceList returns the accumulated advice from this rule
	GetAdviceList() []*types.Advice
}

// GenericChecker embeds the base MySQL parser listener and dispatches events to registered rules.
// This design ensures only one copy of the listener type metadata in the binary.
type GenericChecker struct {
	*mysql.BaseMySQLParserListener

	rules    []Rule
	baseLine int
}

// NewGenericChecker creates a new instance of GenericChecker with the given rules.
func NewGenericChecker(rules []Rule) *GenericChecker {
	return &GenericChecker{
		rules: rules,
	}
}

// NewGenericAntlrChecker creates a new instance of GenericChecker with ANTLR rules.
func NewGenericAntlrChecker(rules []AntlrRule) *GenericChecker {
	var genericRules []Rule
	for _, rule := range rules {
		genericRules = append(genericRules, rule)
	}
	return &GenericChecker{
		rules: genericRules,
	}
}

// SetBaseLine sets the base line number for error reporting.
func (g *GenericChecker) SetBaseLine(baseLine int) {
	g.baseLine = baseLine
}

// GetBaseLine returns the current base line number.
func (g *GenericChecker) GetBaseLine() int {
	return g.baseLine
}

// EnterEveryRule is called when any rule is entered.
// It dispatches the event to all registered rules.
func (g *GenericChecker) EnterEveryRule(ctx antlr.ParserRuleContext) {
	nodeType := g.getNodeType(ctx)
	for _, rule := range g.rules {
		if err := rule.OnEnter(ctx, nodeType); err != nil {
			// Log error using slog for library-friendly logging
			slog.Debug("Rule error on enter",
				"rule", rule.Name(),
				"nodeType", nodeType,
				"error", err)
		}
	}
}

// ExitEveryRule is called when any rule is exited.
// It dispatches the event to all registered rules.
func (g *GenericChecker) ExitEveryRule(ctx antlr.ParserRuleContext) {
	nodeType := g.getNodeType(ctx)
	for _, rule := range g.rules {
		if err := rule.OnExit(ctx, nodeType); err != nil {
			// Log error using slog for library-friendly logging
			slog.Debug("Rule error on exit",
				"rule", rule.Name(),
				"nodeType", nodeType,
				"error", err)
		}
	}
}

// GetAdviceList collects and returns all advice from registered rules.
func (g *GenericChecker) GetAdviceList() []*types.Advice {
	var allAdvice []*types.Advice
	for _, rule := range g.rules {
		allAdvice = append(allAdvice, rule.GetAdviceList()...)
	}
	return allAdvice
}

// getNodeType returns the type name of the parse tree node.
func (*GenericChecker) getNodeType(ctx antlr.ParserRuleContext) string {
	t := reflect.TypeOf(ctx)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := t.Name()
	// Remove "Context" suffix if present
	name = strings.TrimSuffix(name, "Context")
	return name
}

// BaseAntlrRule provides common functionality for ANTLR rules.
// Other rules can embed this struct to get common behavior.
type BaseAntlrRule struct {
	level      types.SQLReviewRuleLevel
	title      string
	adviceList []*types.Advice
	baseLine   int
}

// SetBaseLine sets the base line for the rule.
func (r *BaseAntlrRule) SetBaseLine(baseLine int) {
	r.baseLine = baseLine
}

// GetAdviceList returns the accumulated advice.
func (r *BaseAntlrRule) GetAdviceList() []*types.Advice {
	return r.adviceList
}

// AddAdvice adds a new advice to the list.
func (r *BaseAntlrRule) AddAdvice(advice *types.Advice) {
	r.adviceList = append(r.adviceList, advice)
}

// ConvertANTLRLineToPosition converts ANTLR line number to Position
func ConvertANTLRLineToPosition(line int) *types.Position {
	pLine := ConvertANTLRLineToPositionLine(line)
	return &types.Position{
		Line: int32(pLine),
	}
}

func ConvertANTLRLineToPositionLine(line int) int {
	positionLine := line - 1
	if line == 0 {
		positionLine = 0
	}
	return positionLine
}

// tableColumnTypes tracks table column types across statements
// tableName --> columnName --> columnType
type tableColumnTypes map[string]map[string]string

func (t tableColumnTypes) set(tableName string, columnName string, columnType string) {
	if _, ok := t[tableName]; !ok {
		t[tableName] = make(map[string]string)
	}
	t[tableName][columnName] = columnType
}

func (t tableColumnTypes) get(tableName string, columnName string) (columnType string, ok bool) {
	if _, ok := t[tableName]; !ok {
		return "", false
	}
	col, ok := t[tableName][columnName]
	return col, ok
}

func (t tableColumnTypes) delete(tableName string, columnName string) {
	if _, ok := t[tableName]; !ok {
		return
	}
	delete(t[tableName], columnName)
}

// Wrapper functions for normalize functions to simplify calls
func NormalizeMySQLTableName(ctx mysql.ITableNameContext) string {
	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx)
	return tableName
}

func NormalizeMySQLTableRef(ctx mysql.ITableRefContext) string {
	_, tableName := mysqlparser.NormalizeMySQLTableRef(ctx)
	return tableName
}

func NormalizeMySQLColumnName(ctx mysql.IColumnNameContext) string {
	_, _, columnName := mysqlparser.NormalizeMySQLColumnName(ctx)
	return columnName
}

func NormalizeMySQLIdentifier(ctx mysql.IIdentifierContext) string {
	return mysqlparser.NormalizeMySQLIdentifier(ctx)
}

func NormalizeMySQLColumnInternalRef(ctx mysql.IColumnInternalRefContext) string {
	return mysqlparser.NormalizeMySQLColumnInternalRef(ctx)
}

func NormalizeKeyListVariants(ctx mysql.IKeyListVariantsContext) []string {
	return mysqlparser.NormalizeKeyListVariants(ctx)
}

func NormalizeKeyList(ctx mysql.IKeyListContext) []string {
	return mysqlparser.NormalizeKeyList(ctx)
}

// ConvertSyntaxErrorToAdvice converts parser syntax errors to advice format like Bytebase does
func ConvertSyntaxErrorToAdvice(err error) ([]*types.Advice, error) {
	if syntaxErr, ok := err.(*mysqlparser.SyntaxError); ok {
		return []*types.Advice{
			{
				Status:        types.Advice_ERROR,
				Code:          int32(types.StatementSyntaxError),
				Title:         "Syntax error",
				Content:       syntaxErr.Message,
				StartPosition: syntaxErr.Position,
			},
		}, nil
	}
	// For other errors, return internal error
	return []*types.Advice{
		{
			Status:  types.Advice_ERROR,
			Code:    int32(types.Internal),
			Title:   "Parse error",
			Content: err.Error(),
		},
	}, nil
}

func NormalizeMySQLDataType(ctx mysql.IDataTypeContext) string {
	return mysqlparser.NormalizeMySQLDataType(ctx, true)
}

// BaseRule provides common functionality for rules using the generic checker approach.
// This matches the backend implementation pattern.
type BaseRule struct {
	level      types.Advice_Status
	title      string
	adviceList []*types.Advice
	baseLine   int
}

// SetBaseLine sets the base line for the rule.
func (r *BaseRule) SetBaseLine(baseLine int) {
	r.baseLine = baseLine
}

// GetAdviceList returns the accumulated advice.
func (r *BaseRule) GetAdviceList() []*types.Advice {
	return r.adviceList
}

// AddAdvice adds a new advice to the list.
func (r *BaseRule) AddAdvice(advice *types.Advice) {
	r.adviceList = append(r.adviceList, advice)
}
