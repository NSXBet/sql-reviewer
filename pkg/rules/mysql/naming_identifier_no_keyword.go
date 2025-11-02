package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// MySQL keywords that should be avoided as identifiers
var mysqlKeywords = map[string]bool{
	"accessible": true, "add": true, "all": true, "alter": true, "analyze": true, "and": true, "as": true, "asc": true,
	"asensitive": true, "before": true, "between": true, "bigint": true, "binary": true, "blob": true, "both": true,
	"by": true, "call": true, "cascade": true, "case": true, "change": true, "char": true, "character": true,
	"check": true, "collate": true, "column": true, "condition": true, "constraint": true, "continue": true,
	"convert": true, "create": true, "cross": true, "current_date": true, "current_time": true, "current_timestamp": true,
	"current_user": true, "cursor": true, "database": true, "databases": true, "day_hour": true, "day_microsecond": true,
	"day_minute": true, "day_second": true, "dec": true, "decimal": true, "declare": true, "default": true,
	"delayed": true, "delete": true, "desc": true, "describe": true, "deterministic": true, "distinct": true,
	"distinctrow": true, "div": true, "double": true, "drop": true, "dual": true, "each": true, "else": true,
	"elseif": true, "enclosed": true, "escaped": true, "exists": true, "exit": true, "explain": true, "false": true,
	"fetch": true, "float": true, "float4": true, "float8": true, "for": true, "force": true, "foreign": true,
	"from": true, "fulltext": true, "grant": true, "group": true, "having": true, "high_priority": true,
	"hour_microsecond": true, "hour_minute": true, "hour_second": true, "if": true, "ignore": true, "in": true,
	"index": true, "infile": true, "inner": true, "inout": true, "insensitive": true, "insert": true, "int": true,
	"int1": true, "int2": true, "int3": true, "int4": true, "int8": true, "integer": true, "interval": true,
	"into": true, "is": true, "iterate": true, "join": true, "key": true, "keys": true, "kill": true,
	"leading": true, "leave": true, "left": true, "like": true, "limit": true, "linear": true, "lines": true,
	"load": true, "localtime": true, "localtimestamp": true, "lock": true, "long": true, "longblob": true,
	"longtext": true, "loop": true, "low_priority": true, "match": true, "mediumblob": true, "mediumint": true,
	"mediumtext": true, "middleint": true, "minute_microsecond": true, "minute_second": true, "mod": true,
	"modifies": true, "natural": true, "not": true, "no_write_to_binlog": true, "null": true, "numeric": true,
	"on": true, "optimize": true, "option": true, "optionally": true, "or": true, "order": true, "out": true,
	"outer": true, "outfile": true, "precision": true, "primary": true, "procedure": true, "purge": true,
	"range": true, "read": true, "reads": true, "read_write": true, "real": true, "references": true,
	"regexp": true, "release": true, "rename": true, "repeat": true, "replace": true, "require": true,
	"restrict": true, "return": true, "revoke": true, "right": true, "rlike": true, "schema": true,
	"schemas": true, "second_microsecond": true, "select": true, "sensitive": true, "separator": true,
	"set": true, "show": true, "smallint": true, "spatial": true, "specific": true, "sql": true,
	"sqlexception": true, "sqlstate": true, "sqlwarning": true, "sql_big_result": true, "sql_calc_found_rows": true,
	"sql_small_result": true, "ssl": true, "starting": true, "straight_join": true, "table": true, "terminated": true,
	"text": true, "then": true, "tinyblob": true, "tinyint": true, "tinytext": true, "to": true, "trailing": true,
	"trigger": true, "true": true, "undo": true, "union": true, "unique": true, "unlock": true, "unsigned": true,
	"update": true, "usage": true, "use": true, "using": true, "utc_date": true, "utc_time": true,
	"utc_timestamp": true, "values": true, "varbinary": true, "varchar": true, "varcharacter": true,
	"varying": true, "when": true, "where": true, "while": true, "with": true, "write": true, "x509": true,
	"xor": true, "year_month": true, "zerofill": true,
	// Additional MySQL 8.0 keywords
	"execute": true, "recursive": true, "lateral": true, "except": true, "intersect": true, "window": true,
	"over": true, "generated": true, "stored": true, "virtual": true, "admin": true, "array": true,
	"clone": true, "cube": true, "cume_dist": true, "dense_rank": true, "empty": true, "first_value": true,
	"grouping": true, "groups": true, "json_table": true, "lag": true, "last_value": true, "lead": true,
	"nth_value": true, "ntile": true, "of": true, "percent_rank": true, "rank": true, "row_number": true,
	"system": true,
}

// NamingIdentifierNoKeywordRule is the ANTLR-based implementation for checking identifier naming convention without keywords
type NamingIdentifierNoKeywordRule struct {
	BaseAntlrRule
}

// NewNamingIdentifierNoKeywordRule creates a new ANTLR-based identifier no keyword rule
func NewNamingIdentifierNoKeywordRule(level types.SQLReviewRuleLevel, title string) *NamingIdentifierNoKeywordRule {
	return &NamingIdentifierNoKeywordRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
	}
}

// Name returns the rule name
func (*NamingIdentifierNoKeywordRule) Name() string {
	return "NamingIdentifierNoKeywordRule"
}

// OnEnter is called when entering a parse tree node
func (r *NamingIdentifierNoKeywordRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypePureIdentifier:
		r.checkPureIdentifier(ctx.(*mysql.PureIdentifierContext))
	case NodeTypeIdentifierKeyword:
		r.checkIdentifierKeyword(ctx.(*mysql.IdentifierKeywordContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*NamingIdentifierNoKeywordRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *NamingIdentifierNoKeywordRule) checkPureIdentifier(ctx *mysql.PureIdentifierContext) {
	// The suspect identifier should be always wrapped in backquotes, otherwise a syntax error will be thrown before entering this checker.
	textNode := ctx.BACK_TICK_QUOTED_ID()
	if textNode == nil {
		return
	}

	// Remove backticks as possible.
	identifier := trimBackTicks(textNode.GetText())
	advice := r.checkIdentifier(identifier)
	if advice != nil {
		advice.StartPosition = ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine())
		r.AddAdvice(advice)
	}
}

func (r *NamingIdentifierNoKeywordRule) checkIdentifierKeyword(ctx *mysql.IdentifierKeywordContext) {
	identifier := ctx.GetText()
	advice := r.checkIdentifier(identifier)
	if advice != nil {
		advice.StartPosition = ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine())
		r.AddAdvice(advice)
	}
}

func (r *NamingIdentifierNoKeywordRule) checkIdentifier(identifier string) *types.Advice {
	if isKeyword(identifier) {
		return &types.Advice{
			Status:  types.Advice_Status(r.level),
			Code:    int32(types.NameIsKeywordIdentifier),
			Title:   r.title,
			Content: fmt.Sprintf("Identifier %q is a keyword and should be avoided", identifier),
		}
	}

	return nil
}

func trimBackTicks(s string) string {
	if len(s) < 2 {
		return s
	}
	return s[1 : len(s)-1]
}

func isKeyword(identifier string) bool {
	return mysqlKeywords[strings.ToLower(identifier)]
}

// NamingIdentifierNoKeywordAdvisor is the advisor using ANTLR parser for identifier no keyword checking
type NamingIdentifierNoKeywordAdvisor struct{}

// Check performs the ANTLR-based identifier no keyword check
func (a *NamingIdentifierNoKeywordAdvisor) Check(
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

	// Create the rule
	namingRule := NewNamingIdentifierNoKeywordRule(types.SQLReviewRuleLevel(level), string(rule.Type))

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{namingRule})

	for _, stmtNode := range root {
		namingRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
