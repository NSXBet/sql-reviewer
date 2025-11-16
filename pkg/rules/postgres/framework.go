package postgres

import (
	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// BasePostgreSQLParserListener is the base listener for PostgreSQL parser.
// It provides common functionality for all PostgreSQL rule checkers.
type BasePostgreSQLParserListener struct {
	*parser.BasePostgreSQLParserListener
}

// isTopLevel checks if the context is at the top level of the parse tree.
// This is used to filter out nested statements that should not be checked.
func isTopLevel(ctx antlr.Tree) bool {
	if ctx == nil {
		return true
	}

	switch ctx := ctx.(type) {
	case *parser.RootContext, *parser.StmtblockContext:
		return true
	case *parser.StmtmultiContext, *parser.StmtContext:
		return isTopLevel(ctx.GetParent())
	default:
		return false
	}
}

// ConvertANTLRLineToPosition converts ANTLR line number to Position.
func ConvertANTLRLineToPosition(line int) *types.Position {
	positionLine := line - 1
	if line == 0 {
		positionLine = 0
	}
	return &types.Position{
		Line: int32(positionLine),
	}
}

// GetTableNameFromContext extracts the table name from a qualified name context.
// Returns (schemaName, tableName).
func GetTableNameFromContext(ctx parser.IQualified_nameContext) (string, string) {
	if ctx == nil {
		return "", ""
	}

	// Import pgparser for normalization
	parts := []string{}

	// Get colid (first part)
	if ctx.Colid() != nil {
		text := ctx.Colid().GetText()
		parts = append(parts, normalizeIdentifier(text))
	}

	// Get indirection parts
	if ctx.Indirection() != nil {
		for _, indirEl := range ctx.Indirection().AllIndirection_el() {
			if indirEl.Attr_name() != nil {
				attrName := indirEl.Attr_name()
				if attrName.Collabel() != nil {
					text := attrName.Collabel().GetText()
					parts = append(parts, normalizeIdentifier(text))
				}
			}
		}
	}

	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

// normalizeIdentifier normalizes a PostgreSQL identifier.
// Quoted identifiers preserve case, unquoted identifiers are lowercased.
func normalizeIdentifier(text string) string {
	if text == "" {
		return ""
	}

	// If quoted, remove quotes and preserve case
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		// Remove quotes and unescape doubled quotes
		result := text[1 : len(text)-1]
		// Replace "" with "
		return replaceString(result, `""`, `"`)
	}

	// Unquoted identifiers are lowercased
	return toLowerCase(text)
}

// toLowerCase converts a string to lowercase.
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// replaceString replaces all occurrences of old with new in s.
func replaceString(s, old, new string) string {
	result := ""
	for len(s) > 0 {
		idx := indexOfString(s, old)
		if idx == -1 {
			result += s
			break
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
	return result
}

// indexOfString returns the index of the first instance of substr in s.
func indexOfString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// NormalizeSchemaName normalizes a schema name, defaulting to "public".
func NormalizeSchemaName(schemaName string) string {
	if schemaName == "" {
		return "public"
	}
	return schemaName
}

// ConvertSyntaxErrorToAdvice converts PostgreSQL parser syntax errors to
// advice format, matching the pattern used in MySQL rules and Bytebase.
//
// This function handles two types of errors:
//   - *pgparser.SyntaxError: Converts to Advice with error code 201 (StatementSyntaxError)
//     and preserves position information (line/column) from the parser
//   - Other errors: Converts to internal error advice with error code 1 (Internal)
//
// The function always returns ([]*types.Advice, nil) and never returns an error,
// which maintains consistency with the advisor interface contract and allows
// syntax errors to be visible to users as actionable advice.
//
// Example usage in PostgreSQL rules:
//
//	tree, err := getANTLRTree(checkCtx)
//	if err != nil {
//	    return ConvertSyntaxErrorToAdvice(err)
//	}
func ConvertSyntaxErrorToAdvice(err error) ([]*types.Advice, error) {
	// Handle nil error (should not happen in practice, but be defensive)
	if err == nil {
		return []*types.Advice{
			{
				Status:        types.Advice_ERROR,
				Code:          int32(types.Internal), // 1
				Title:         "Parse error",
				Content:       "unknown error (nil)",
				StartPosition: nil,
			},
		}, nil
	}

	// Handle typed syntax errors with position information
	if syntaxErr, ok := err.(*pgparser.SyntaxError); ok {
		return []*types.Advice{
			{
				Status:        types.Advice_ERROR,
				Code:          int32(types.StatementSyntaxError), // 201
				Title:         advisor.SyntaxErrorTitle,          // "Syntax error"
				Content:       syntaxErr.Message,
				StartPosition: syntaxErr.Position,
			},
		}, nil
	}

	// Fallback for unexpected error types
	return []*types.Advice{
		{
			Status:        types.Advice_ERROR,
			Code:          int32(types.Internal), // 1
			Title:         "Parse error",
			Content:       err.Error(),
			StartPosition: nil,
		},
	}, nil
}
