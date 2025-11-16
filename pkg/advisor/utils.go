package advisor

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/nsxbet/sql-reviewer/pkg/types"
	"github.com/pkg/errors"
)

// NormalizeStatement formats and limits the max length of SQL statements for logging.
// It removes extra whitespace, normalizes line breaks, and truncates if too long.
func NormalizeStatement(statement string) string {
	// Remove leading/trailing whitespace
	statement = strings.TrimSpace(statement)

	// Replace multiple spaces/tabs with single space
	statement = regexp.MustCompile(`[\t ]+`).ReplaceAllString(statement, " ")

	// Replace multiple newlines with single newline
	statement = regexp.MustCompile(`\n+`).ReplaceAllString(statement, "\n")

	// Remove newlines followed by spaces
	statement = regexp.MustCompile(`\n\s+`).ReplaceAllString(statement, "\n")

	// For single-line queries, keep them on one line
	if !strings.Contains(statement, "\n") {
		maxLength := 1000
		if len(statement) > maxLength {
			return statement[:maxLength] + "..."
		}
		return statement
	}

	// For multi-line queries, format nicely with proper indentation
	lines := strings.Split(statement, "\n")
	var formatted []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			formatted = append(formatted, line)
		}
	}
	statement = strings.Join(formatted, "\n")

	// Truncate if too long (accounting for newlines)
	maxLength := 2000
	if len(statement) > maxLength {
		// Find a good break point (end of line if possible)
		truncated := statement[:maxLength]
		if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxLength-200 {
			truncated = truncated[:lastNewline]
		}
		return truncated + "\n..."
	}

	return statement
}

// getSQLColor returns the ANSI color code for a SQL statement based on Rails 5+ conventions.
// The color is determined by the statement type (SELECT, INSERT, UPDATE, DELETE, etc.).
func getSQLColor(statement string) string {
	// Rails 5+ SQL color standard
	const (
		colorBlue    = "\033[34m" // SELECT
		colorGreen   = "\033[32m" // INSERT
		colorYellow  = "\033[33m" // UPDATE
		colorRed     = "\033[31m" // DELETE, ROLLBACK
		colorCyan    = "\033[36m" // TRANSACTION, BEGIN, COMMIT
		colorMagenta = "\033[35m" // EXPLAIN, DDL (CREATE, ALTER, DROP)
		colorWhite   = "\033[37m" // LOCK, SELECT FOR UPDATE
	)

	// Normalize for pattern matching
	upperStatement := strings.ToUpper(strings.TrimSpace(statement))

	// Check for ROLLBACK (must come before other patterns)
	if strings.HasPrefix(upperStatement, "ROLLBACK") {
		return colorRed
	}

	// Check for SELECT ... FOR UPDATE or LOCK
	if strings.HasPrefix(upperStatement, "LOCK") ||
		(strings.HasPrefix(upperStatement, "SELECT") && strings.Contains(upperStatement, "FOR UPDATE")) {
		return colorWhite
	}

	// Check for basic DML operations
	if strings.HasPrefix(upperStatement, "SELECT") {
		return colorBlue
	}
	if strings.HasPrefix(upperStatement, "INSERT") {
		return colorGreen
	}
	if strings.HasPrefix(upperStatement, "UPDATE") {
		return colorYellow
	}
	if strings.HasPrefix(upperStatement, "DELETE") {
		return colorRed
	}

	// Check for transaction control
	if strings.HasPrefix(upperStatement, "BEGIN") ||
		strings.HasPrefix(upperStatement, "COMMIT") ||
		strings.Contains(upperStatement, "TRANSACTION") {
		return colorCyan
	}

	// Check for EXPLAIN
	if strings.HasPrefix(upperStatement, "EXPLAIN") {
		return colorMagenta
	}

	// Check for DDL operations (CREATE, ALTER, DROP)
	if strings.HasPrefix(upperStatement, "CREATE") ||
		strings.HasPrefix(upperStatement, "ALTER") ||
		strings.HasPrefix(upperStatement, "DROP") {
		return colorMagenta
	}

	// Default to magenta for other statements
	return colorMagenta
}

// formatSQLForLog formats SQL statements for beautiful log output.
// It adds proper indentation, highlights SQL keywords, and colors the output
// using Rails 5+ color conventions based on statement type.
func formatSQLForLog(statement string) string {
	// Normalize first
	statement = NormalizeStatement(statement)

	// Get color based on statement type (Rails 5+ convention)
	color := getSQLColor(statement)
	const colorReset = "\033[0m" // Reset color

	// For short single-line statements, wrap in color and return
	if !strings.Contains(statement, "\n") && len(statement) < 100 {
		return color + statement + colorReset
	}

	// Split into lines and add indentation
	lines := strings.Split(statement, "\n")
	var formatted []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Add indentation for readability
		// Main clauses (SELECT, FROM, WHERE, etc.) stay at base level
		// Everything else gets indented
		mainClauses := []string{"SELECT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER",
			"GROUP BY", "ORDER BY", "HAVING", "LIMIT", "OFFSET", "INSERT", "UPDATE",
			"DELETE", "VALUES", "SET", "ON", "AND", "OR", "UNION", "EXPLAIN"}

		isMainClause := false
		upperLine := strings.ToUpper(line)
		for _, clause := range mainClauses {
			if strings.HasPrefix(upperLine, clause) {
				isMainClause = true
				break
			}
		}

		if isMainClause {
			formatted = append(formatted, line)
		} else {
			// Check if line starts with opening paren - don't indent
			if strings.HasPrefix(line, "(") {
				formatted = append(formatted, line)
			} else {
				formatted = append(formatted, "  "+line)
			}
		}
	}

	// Wrap the entire formatted SQL in statement-specific color
	return color + strings.Join(formatted, "\n") + colorReset
}

type QueryContext struct {
	UsePostgresDatabaseOwner bool
	PreExecutions            []string
}

// Query runs the EXPLAIN or SELECT statements for advisors.
func Query(ctx context.Context, qCtx QueryContext, connection *sql.DB, engine types.Engine, statement string) ([]any, error) {
	// Log query start
	startTime := time.Now()
	slog.Debug("Starting SQL query",
		"engine", engine,
		"query", formatSQLForLog(statement),
		"pre_executions", qCtx.PreExecutions,
		"use_db_owner", qCtx.UsePostgresDatabaseOwner,
	)

	tx, err := connection.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		slog.Debug("Failed to begin transaction", "error", err)
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
		slog.Debug("Transaction rolled back")
	}()

	slog.Debug("Transaction started")

	if engine == types.Engine_POSTGRES && qCtx.UsePostgresDatabaseOwner {
		const query = `
		SELECT
			u.rolname
		FROM
			pg_roles AS u JOIN pg_database AS d ON (d.datdba = u.oid)
		WHERE
			d.datname = current_database();
		`
		slog.Debug("Querying database owner role")
		var owner string
		if err := tx.QueryRowContext(ctx, query).Scan(&owner); err != nil {
			slog.Debug("Failed to query database owner", "error", err)
			return nil, err
		}
		setRoleStmt := fmt.Sprintf("SET ROLE '%s';", owner)
		slog.Debug("Setting database owner role", "owner", owner, "statement", setRoleStmt)
		if _, err := tx.ExecContext(ctx, setRoleStmt); err != nil {
			slog.Debug("Failed to set role", "error", err)
			return nil, err
		}
	}

	for i, preExec := range qCtx.PreExecutions {
		if preExec != "" {
			slog.Debug("Executing pre-execution statement", "index", i, "statement", formatSQLForLog(preExec))
			if _, err := tx.ExecContext(ctx, preExec); err != nil {
				slog.Debug("Pre-execution failed", "index", i, "error", err)
				return nil, errors.Wrapf(err, "failed to execute pre-execution: %s", preExec)
			}
			slog.Debug("Pre-execution completed", "index", i)
		}
	}

	slog.Debug("Executing main query", "statement", formatSQLForLog(statement))
	rows, err := tx.QueryContext(ctx, statement)
	if err != nil {
		slog.Debug("Query execution failed", "error", err)
		return nil, err
	}
	slog.Debug("Query execution succeeded")
	defer func() {
		_ = rows.Close()
	}()

	columnNames, err := rows.Columns()
	if err != nil {
		slog.Debug("Failed to get column names", "error", err)
		return nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		slog.Debug("Failed to get column types", "error", err)
		return nil, err
	}

	colCount := len(columnTypes)
	slog.Debug("Query result metadata", "column_count", colCount, "column_names", columnNames)

	var columnTypeNames []string
	for _, v := range columnTypes {
		// DatabaseTypeName returns the database system name of the column type.
		// refer: https://pkg.go.dev/database/sql#ColumnType.DatabaseTypeName
		columnTypeNames = append(columnTypeNames, strings.ToUpper(v.DatabaseTypeName()))
	}

	data := []any{}
	rowCount := 0
	for rows.Next() {
		rowCount++
		scanArgs := make([]any, colCount)
		for i, v := range columnTypeNames {
			// TODO(steven need help): Consult a common list of data types from database driver documentation. e.g. MySQL,PostgreSQL.
			switch v {
			case "VARCHAR", "TEXT", "UUID", "TIMESTAMP":
				scanArgs[i] = new(sql.NullString)
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
			case "INT", "INTEGER":
				scanArgs[i] = new(sql.NullInt64)
			case "FLOAT":
				scanArgs[i] = new(sql.NullFloat64)
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}

		rowData := []any{}
		for i := range columnTypes {
			if v, ok := (scanArgs[i]).(*sql.NullBool); ok && v.Valid {
				rowData = append(rowData, v.Bool)
				continue
			}
			if v, ok := (scanArgs[i]).(*sql.NullString); ok && v.Valid {
				rowData = append(rowData, v.String)
				continue
			}
			if v, ok := (scanArgs[i]).(*sql.NullInt64); ok && v.Valid {
				rowData = append(rowData, v.Int64)
				continue
			}
			if v, ok := (scanArgs[i]).(*sql.NullInt32); ok && v.Valid {
				rowData = append(rowData, v.Int32)
				continue
			}
			if v, ok := (scanArgs[i]).(*sql.NullFloat64); ok && v.Valid {
				rowData = append(rowData, v.Float64)
				continue
			}
			// If none of them match, set nil to its value.
			rowData = append(rowData, nil)
		}

		data = append(data, rowData)
	}
	if err := rows.Err(); err != nil {
		slog.Debug("Error iterating rows", "error", err)
		return nil, err
	}

	duration := time.Since(startTime)
	slog.Debug("Query completed successfully",
		"duration_ms", duration.Milliseconds(),
		"row_count", rowCount,
		"column_count", colCount,
	)

	return []any{columnNames, columnTypeNames, data}, nil
}

func DatabaseExists(ctx context.Context, checkCtx SQLReviewCheckContext, database string) bool {
	if checkCtx.ListDatabaseNamesFunc == nil {
		return false
	}

	names, err := checkCtx.ListDatabaseNamesFunc(ctx, checkCtx.InstanceID)
	if err != nil {
		slog.Debug("failed to list databases", slog.String("instance", checkCtx.InstanceID), slog.Any("error", err))
		return false
	}

	for _, name := range names {
		if name == database {
			return true
		}
	}

	return false
}

// Additional utility functions that might be needed for rule implementations

// UnmarshalNumberTypeRulePayload unmarshals a number type rule payload
func UnmarshalNumberTypeRulePayload(payload map[string]interface{}) (*NumberTypeRulePayload, error) {
	if payload == nil {
		return nil, errors.New("payload is nil")
	}

	number, ok := payload["number"]
	if !ok {
		return nil, errors.New("missing 'number' field in payload")
	}

	var num int
	switch v := number.(type) {
	case int:
		num = v
	case float64:
		num = int(v)
	case string:
		// Try to parse string as int if needed
		return nil, errors.New("string number parsing not implemented")
	default:
		return nil, errors.New("invalid number type in payload")
	}

	return &NumberTypeRulePayload{Number: num}, nil
}

// NumberTypeRulePayload represents a payload with a number field
type NumberTypeRulePayload struct {
	Number int `json:"number"`
}

// UnmarshalStringArrayTypeRulePayload unmarshals a string array type rule payload
func UnmarshalStringArrayTypeRulePayload(payload map[string]interface{}) (*StringArrayTypeRulePayload, error) {
	if payload == nil {
		return nil, errors.New("payload is nil")
	}

	listInterface, ok := payload["list"]
	if !ok {
		return nil, errors.New("missing 'list' field in payload")
	}

	// Handle both []interface{} and []string cases, and nil (empty array) case
	var list []string
	switch v := listInterface.(type) {
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				list = append(list, str)
			} else {
				return nil, errors.New("non-string item in list")
			}
		}
	case []string:
		list = v
	case nil:
		// Empty array case - this is valid
		list = []string{}
	default:
		return nil, errors.New("'list' field is not an array")
	}

	return &StringArrayTypeRulePayload{List: list}, nil
}

// StringArrayTypeRulePayload represents a payload with a string array field
type StringArrayTypeRulePayload struct {
	List []string `json:"list"`
}

// UnmarshalStringTypeRulePayload unmarshals a string type rule payload
func UnmarshalStringTypeRulePayload(payload map[string]interface{}) (*StringTypeRulePayload, error) {
	if payload == nil {
		return nil, errors.New("payload is nil")
	}

	stringInterface, ok := payload["string"]
	if !ok {
		return nil, errors.New("missing 'string' field in payload")
	}

	str, ok := stringInterface.(string)
	if !ok {
		return nil, errors.New("'string' field is not a string")
	}

	return &StringTypeRulePayload{String: str}, nil
}

// StringTypeRulePayload represents a payload with a string field
type StringTypeRulePayload struct {
	String string `json:"string"`
}

// UnmarshalBooleanTypeRulePayload unmarshals a boolean type rule payload
func UnmarshalBooleanTypeRulePayload(payload map[string]interface{}) (*BooleanTypeRulePayload, error) {
	if payload == nil {
		return nil, errors.New("payload is nil")
	}

	boolInterface, ok := payload["boolean"]
	if !ok {
		return nil, errors.New("missing 'boolean' field in payload")
	}

	boolVal, ok := boolInterface.(bool)
	if !ok {
		return nil, errors.New("'boolean' field is not a boolean")
	}

	return &BooleanTypeRulePayload{Boolean: boolVal}, nil
}

// BooleanTypeRulePayload represents a payload with a boolean field
type BooleanTypeRulePayload struct {
	Boolean bool `json:"boolean"`
}

// NamingRulePayload represents a payload for naming rules with format and maxLength
type NamingRulePayload struct {
	Format    string `json:"format"`
	MaxLength int    `json:"maxLength"`
}

// UnmarshalNamingRulePayload unmarshals a naming rule payload with format and maxLength
func UnmarshalNamingRulePayload(payload map[string]interface{}) (*NamingRulePayload, error) {
	if payload == nil {
		return nil, errors.New("payload is nil")
	}

	result := &NamingRulePayload{}

	// Extract format
	if formatInterface, ok := payload["format"]; ok {
		if formatStr, ok := formatInterface.(string); ok {
			result.Format = formatStr
		} else {
			return nil, errors.New("'format' field is not a string")
		}
	} else {
		return nil, errors.New("missing 'format' field in payload")
	}

	// Extract maxLength (optional)
	if maxLengthInterface, ok := payload["maxLength"]; ok {
		switch v := maxLengthInterface.(type) {
		case int:
			result.MaxLength = v
		case float64:
			result.MaxLength = int(v)
		default:
			return nil, errors.New("'maxLength' field is not a number")
		}
	}

	return result, nil
}

// NamingRulePayloadRegexp represents a naming rule payload with compiled regexp
type NamingRulePayloadRegexp struct {
	Format       *regexp.Regexp
	FormatString string // Original format string for error messages
	MaxLength    int
}

// UnmarshalNamingRulePayloadAsRegexp unmarshals a naming rule payload and returns a compiled regexp
func UnmarshalNamingRulePayloadAsRegexp(payload map[string]interface{}) (*NamingRulePayloadRegexp, error) {
	result, err := UnmarshalNamingRulePayload(payload)
	if err != nil {
		return nil, err
	}

	// Compile the format as a regular expression
	format, err := regexp.Compile(result.Format)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regular expression %q", result.Format)
	}

	return &NamingRulePayloadRegexp{
		Format:       format,
		FormatString: result.Format,
		MaxLength:    result.MaxLength,
	}, nil
}

// UnmarshalNamingRulePayloadAsTemplate unmarshals a naming rule payload and returns format template, template list, and maxLength.
// This is used for template-based naming rules that support token replacement (e.g., {{table}}, {{column_list}}).
func UnmarshalNamingRulePayloadAsTemplate(payload map[string]interface{}) (string, []string, int, error) {
	if payload == nil {
		return "", nil, 0, errors.New("payload is nil")
	}

	var format string
	var templateList []string
	var maxLength int

	// Extract format (string template)
	if formatInterface, ok := payload["format"]; ok {
		if formatStr, ok := formatInterface.(string); ok {
			format = formatStr
		} else {
			return "", nil, 0, errors.New("'format' field is not a string")
		}
	}

	// Extract templateList (array of string templates)
	if templateListInterface, ok := payload["templateList"]; ok {
		switch v := templateListInterface.(type) {
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					templateList = append(templateList, str)
				} else {
					return "", nil, 0, errors.New("'templateList' contains non-string element")
				}
			}
		case []string:
			templateList = v
		default:
			return "", nil, 0, errors.New("'templateList' field is not an array")
		}
	}

	// At least one of format or templateList must be provided
	if format == "" && len(templateList) == 0 {
		return "", nil, 0, errors.New("either 'format' or 'templateList' must be provided")
	}

	// Extract maxLength (optional)
	if maxLengthInterface, ok := payload["maxLength"]; ok {
		switch v := maxLengthInterface.(type) {
		case int:
			maxLength = v
		case float64:
			maxLength = int(v)
		default:
			return "", nil, 0, errors.New("'maxLength' field is not a number")
		}
	}

	return format, templateList, maxLength, nil
}

// CommentConventionRulePayload represents a payload for comment convention rules
type CommentConventionRulePayload struct {
	Required               bool `json:"required"`
	RequiredClassification bool `json:"requiredClassification"`
	MaxLength              int  `json:"maxLength"`
}

// UnmarshalCommentConventionRulePayload unmarshals a comment convention rule payload
func UnmarshalCommentConventionRulePayload(payload map[string]interface{}) (*CommentConventionRulePayload, error) {
	if payload == nil {
		return nil, errors.New("payload is nil")
	}

	result := &CommentConventionRulePayload{}

	// Extract required (optional)
	if requiredInterface, ok := payload["required"]; ok {
		if required, ok := requiredInterface.(bool); ok {
			result.Required = required
		} else {
			return nil, errors.New("'required' field is not a boolean")
		}
	}

	// Extract requiredClassification (optional)
	if requiredClassificationInterface, ok := payload["requiredClassification"]; ok {
		if requiredClassification, ok := requiredClassificationInterface.(bool); ok {
			result.RequiredClassification = requiredClassification
		} else {
			return nil, errors.New("'requiredClassification' field is not a boolean")
		}
	}

	// Extract maxLength (optional)
	if maxLengthInterface, ok := payload["maxLength"]; ok {
		switch v := maxLengthInterface.(type) {
		case int:
			result.MaxLength = v
		case float64:
			result.MaxLength = int(v)
		default:
			return nil, errors.New("'maxLength' field is not a number")
		}
	}

	return result, nil
}

// UnmarshalRequiredColumnList unmarshals a payload and parses the required column list
func UnmarshalRequiredColumnList(payload map[string]interface{}) ([]string, error) {
	stringArrayRulePayload, err := UnmarshalStringArrayTypeRulePayload(payload)
	if err != nil {
		return nil, err
	}
	if len(stringArrayRulePayload.List) != 0 {
		return stringArrayRulePayload.List, nil
	}
	return nil, nil
}
