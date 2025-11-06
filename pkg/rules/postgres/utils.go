package postgres

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
	"github.com/pkg/errors"
)

// getANTLRTree returns the ANTLR parse tree from the advisor context.
func getANTLRTree(checkCtx advisor.Context) (*pgparser.ParseResult, error) {
	if checkCtx.AST == nil {
		// Parse the SQL if not already parsed
		result, err := pgparser.ParsePostgreSQL(checkCtx.Statements)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// Type assert the AST to ParseResult
	result, ok := checkCtx.AST.(*pgparser.ParseResult)
	if !ok {
		return nil, errors.Errorf("unexpected AST type: %T", checkCtx.AST)
	}

	return result, nil
}

// extractTableName extracts the table name from a qualified name context.
func extractTableName(ctx parser.IQualified_nameContext) string {
	if ctx == nil {
		return ""
	}

	parts := pgparser.NormalizePostgreSQLQualifiedName(ctx)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// extractSchemaName extracts the schema name from a qualified name context.
func extractSchemaName(ctx parser.IQualified_nameContext) string {
	if ctx == nil {
		return ""
	}

	parts := pgparser.NormalizePostgreSQLQualifiedName(ctx)
	if len(parts) <= 1 {
		return ""
	}
	return parts[0]
}

// extractIntegerConstant extracts an integer value from an iconst context.
//
//nolint:unused
func extractIntegerConstant(ctx parser.IIconstContext) (int, error) {
	if ctx == nil {
		return 0, errors.New("iconst context is nil")
	}

	text := ctx.GetText()
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse integer constant: %s", text)
	}

	return value, nil
}

// extractStringConstant extracts a string value from an sconst context.
func extractStringConstant(ctx parser.ISconstContext) string {
	if ctx == nil {
		return ""
	}

	text := ctx.GetText()

	// Remove quotes
	if len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
		// Unescape single quotes ('' becomes ')
		unescaped := text[1 : len(text)-1]
		return strings.ReplaceAll(unescaped, "''", "'")
	}

	return text
}

// extractStatementText extracts the text of a statement between two line numbers.
//
//nolint:unused
func extractStatementText(statementsText string, startLine, endLine int) string {
	lines := strings.Split(statementsText, "\n")

	if startLine < 0 || startLine >= len(lines) {
		return ""
	}

	if endLine < 0 || endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	if startLine > endLine {
		return ""
	}

	return strings.Join(lines[startLine:endLine+1], "\n")
}

// getTemplateRegexp generates a regex from a template string with tokens.
// Template example: "{{table}}_{{column}}_idx"
// Tokens example: map[string]string{"table": "users", "column": "id"}
func getTemplateRegexp(template string, templateList []string, tokens map[string]string) (*regexp.Regexp, error) {
	if template != "" {
		// Single template mode
		return compileTemplateRegexp(template, tokens)
	}

	if len(templateList) == 0 {
		return nil, errors.New("both template and templateList are empty")
	}

	// Multiple templates mode - combine with OR
	patterns := make([]string, 0, len(templateList))
	for _, tmpl := range templateList {
		re, err := compileTemplateRegexp(tmpl, tokens)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, re.String())
	}

	combinedPattern := strings.Join(patterns, "|")
	return regexp.Compile(combinedPattern)
}

// compileTemplateRegexp compiles a single template into a regex.
func compileTemplateRegexp(template string, tokens map[string]string) (*regexp.Regexp, error) {
	// Replace tokens with their values or regex patterns
	pattern := template

	// Common tokens
	tokenReplacements := map[string]string{
		"{{table}}":              `[a-z]+(_[a-z]+)*`,
		"{{column}}":             `[a-z]+(_[a-z]+)*`,
		"{{column_list}}":        `[a-z]+(_[a-z]+)*`,
		"{{referencing_table}}":  `[a-z]+(_[a-z]+)*`,
		"{{referencing_column}}": `[a-z]+(_[a-z]+)*`,
		"{{referenced_table}}":   `[a-z]+(_[a-z]+)*`,
		"{{referenced_column}}":  `[a-z]+(_[a-z]+)*`,
	}

	// Apply custom token values if provided
	for token, value := range tokens {
		if _, exists := tokenReplacements[token]; !exists {
			tokenReplacements[token] = regexp.QuoteMeta(value)
		}
	}

	// Replace all tokens
	for token, replacement := range tokenReplacements {
		pattern = strings.ReplaceAll(pattern, token, replacement)
	}

	// Compile the regex
	return regexp.Compile(pattern)
}

// normalizePostgreSQLSchemaTableName normalizes a schema.table qualified name.
//
//nolint:unused
func normalizePostgreSQLSchemaTableName(schemaName, tableName string) string {
	if schemaName == "" {
		return tableName
	}
	return fmt.Sprintf("%s.%s", schemaName, tableName)
}

// Helper function to check if a context has a parent of a specific type.
// Note: This is a placeholder implementation. In practice, you would pass
// a concrete type or use reflection for type checking.
//
//nolint:unused
func hasParentOfType(ctx antlr.Tree) bool {
	if ctx == nil {
		return false
	}

	// This is a simplified version - would need specific type checking
	return ctx.GetParent() != nil
}

// PostgreSQL type equivalence helper functions

// normalizePostgreSQLType normalizes a PostgreSQL type name to its canonical form.
// This handles common aliases and returns the standard representation.
func normalizePostgreSQLType(typeName string) string {
	typeName = strings.ToLower(strings.TrimSpace(typeName))

	// Remove whitespace inside parentheses: " VARCHAR ( 20 ) " -> "varchar(20)"
	typeName = removeWhitespaceInParams(typeName)

	// Handle SERIAL types - they're aliases for INTEGER types with sequences
	switch typeName {
	case "serial":
		return "integer"
	case "bigserial":
		return "bigint"
	case "smallserial":
		return "smallint"
	}

	// Handle VARCHAR - it's an alias for CHARACTER VARYING
	if strings.HasPrefix(typeName, "varchar") {
		if typeName == "varchar" {
			return "character varying"
		}
		// varchar(N) -> character varying(N)
		if strings.HasPrefix(typeName, "varchar(") {
			params := strings.TrimPrefix(typeName, "varchar")
			return "character varying" + params
		}
	}

	return typeName
}

// removeWhitespaceInParams removes whitespace inside parentheses
// " VARCHAR ( 20 ) " -> "varchar(20)"
// "double precision" -> "double precision" (unchanged)
func removeWhitespaceInParams(typeName string) string {
	idx := strings.Index(typeName, "(")
	if idx == -1 {
		// No parameters, just normalize spaces between words to single space
		return normalizeSpaces(typeName)
	}

	// Split into base type and parameters
	baseType := normalizeSpaces(typeName[:idx])
	params := strings.ReplaceAll(typeName[idx:], " ", "")
	return baseType + params
}

// normalizeSpaces normalizes multiple spaces to single space
func normalizeSpaces(s string) string {
	// Replace multiple spaces with single space
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// areTypesEquivalent checks if two PostgreSQL types are equivalent.
// This includes:
// - Exact matches after normalization
// - Type family equivalence (e.g., int/integer/int4)
func areTypesEquivalent(typeA, typeB string) bool {
	typeA = normalizePostgreSQLType(typeA)
	typeB = normalizePostgreSQLType(typeB)

	// Exact match after normalization
	if typeA == typeB {
		return true
	}

	// Check type families
	// Integer family: int, integer, int4
	intTypes := []string{"int", "integer", "int4"}
	if inSlice(typeA, intTypes) && inSlice(typeB, intTypes) {
		return true
	}

	// Bigint family: bigint, int8
	bigintTypes := []string{"bigint", "int8"}
	if inSlice(typeA, bigintTypes) && inSlice(typeB, bigintTypes) {
		return true
	}

	// Smallint family: smallint, int2
	smallintTypes := []string{"smallint", "int2"}
	if inSlice(typeA, smallintTypes) && inSlice(typeB, smallintTypes) {
		return true
	}

	// Real family: real, float4
	realTypes := []string{"real", "float4"}
	if inSlice(typeA, realTypes) && inSlice(typeB, realTypes) {
		return true
	}

	// Double precision family: double precision, float8
	doubleTypes := []string{"double precision", "float8"}
	if inSlice(typeA, doubleTypes) && inSlice(typeB, doubleTypes) {
		return true
	}

	// Boolean family: boolean, bool
	boolTypes := []string{"boolean", "bool"}
	if inSlice(typeA, boolTypes) && inSlice(typeB, boolTypes) {
		return true
	}

	// Character family: character, char (but we need to preserve length parameters)
	// For types with parameters, check base type equivalence
	if hasTypeParameters(typeA) && hasTypeParameters(typeB) {
		baseA, paramsA := splitTypeAndParams(typeA)
		baseB, paramsB := splitTypeAndParams(typeB)

		// Check if base types are equivalent
		if areBaseTypesEquivalent(baseA, baseB) {
			// For character types, parameters must also match
			return paramsA == paramsB
		}
	}

	return false
}

// hasTypeParameters checks if a type has parameters (e.g., varchar(20))
func hasTypeParameters(typeName string) bool {
	return strings.Contains(typeName, "(")
}

// splitTypeAndParams splits a type into base type and parameters
// e.g., "character varying(20)" -> ("character varying", "(20)")
func splitTypeAndParams(typeName string) (string, string) {
	idx := strings.Index(typeName, "(")
	if idx == -1 {
		return typeName, ""
	}
	return strings.TrimSpace(typeName[:idx]), typeName[idx:]
}

// areBaseTypesEquivalent checks if two base types (without parameters) are equivalent
func areBaseTypesEquivalent(baseA, baseB string) bool {
	// Normalize base types
	baseA = strings.ToLower(strings.TrimSpace(baseA))
	baseB = strings.ToLower(strings.TrimSpace(baseB))

	if baseA == baseB {
		return true
	}

	// Character varying family
	charVaryingTypes := []string{"character varying", "varchar"}
	if inSlice(baseA, charVaryingTypes) && inSlice(baseB, charVaryingTypes) {
		return true
	}

	// Character family
	charTypes := []string{"character", "char"}
	if inSlice(baseA, charTypes) && inSlice(baseB, charTypes) {
		return true
	}

	return false
}

// inSlice checks if a string is in a slice
func inSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// isTypeInList checks if a column type matches any type in the given list,
// considering type equivalence.
//
//nolint:unused
func isTypeInList(columnType string, typeList []string) bool {
	for _, listType := range typeList {
		if areTypesEquivalent(columnType, listType) {
			return true
		}
	}
	return false
}

// getLineNumber returns the line number for a parser rule context.
//
//nolint:unused
func getLineNumber(ctx antlr.ParserRuleContext) int {
	if ctx == nil {
		return 0
	}

	token := ctx.GetStart()
	if token == nil {
		return 0
	}

	return token.GetLine()
}

// getColumnNumber returns the column number for a parser rule context.
//
//nolint:unused
func getColumnNumber(ctx antlr.ParserRuleContext) int {
	if ctx == nil {
		return 0
	}

	token := ctx.GetStart()
	if token == nil {
		return 0
	}

	return token.GetColumn()
}

// createAdvice creates an Advice object with the given parameters.
//
//nolint:unused
func createAdvice(status types.Advice_Status, title, content string, line int) *types.Advice {
	return &types.Advice{
		Status:        status,
		Title:         title,
		Content:       content,
		Code:          0, // Will be set by the rule
		StartPosition: ConvertANTLRLineToPosition(line),
	}
}

// createAdviceWithCode creates an Advice object with a specific code.
//
//nolint:unused
func createAdviceWithCode(status types.Advice_Status, code int32, title, content string, line int) *types.Advice {
	return &types.Advice{
		Status:        status,
		Code:          code,
		Title:         title,
		Content:       content,
		StartPosition: ConvertANTLRLineToPosition(line),
	}
}

// getAffectedRows extracts the estimated row count from PostgreSQL EXPLAIN output.
func getAffectedRows(res []any) (int64, error) {
	// the res struct is []any{columnName, columnTable, rowDataList}
	if len(res) != 3 {
		return 0, errors.Errorf("expected 3 but got %d", len(res))
	}
	rowList, ok := res[2].([]any)
	if !ok {
		return 0, errors.Errorf("expected []any but got %T", res[2])
	}
	// EXPLAIN output has at least 2 rows
	if len(rowList) < 2 {
		return 0, errors.New("not found any data")
	}
	// We need row 2
	rowTwo, ok := rowList[1].([]any)
	if !ok {
		return 0, errors.Errorf("expected []any but got %T", rowList[0])
	}
	// PostgreSQL EXPLAIN result has one column
	if len(rowTwo) != 1 {
		return 0, errors.Errorf("expected one but got %d", len(rowTwo))
	}
	// Get the string value
	text, ok := rowTwo[0].(string)
	if !ok {
		return 0, errors.Errorf("expected string but got %T", rowTwo[0])
	}

	rowsRegexp := regexp.MustCompile("rows=([0-9]+)")
	matches := rowsRegexp.FindStringSubmatch(text)
	if len(matches) != 2 {
		return 0, errors.Errorf("failed to find rows in %q", text)
	}
	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, errors.Errorf("failed to get integer from %q", matches[1])
	}
	return value, nil
}
