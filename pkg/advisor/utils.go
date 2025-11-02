package advisor

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nsxbet/sql-reviewer/pkg/types"
	"github.com/pkg/errors"
)

// NormalizeStatement limit the max length of the statements.
func NormalizeStatement(statement string) string {
	maxLength := 1000
	if len(statement) > maxLength {
		return statement[:maxLength] + "..."
	}
	return statement
}

type QueryContext struct {
	UsePostgresDatabaseOwner bool
	PreExecutions            []string
}

// Query runs the EXPLAIN or SELECT statements for advisors.
func Query(ctx context.Context, qCtx QueryContext, connection *sql.DB, engine types.Engine, statement string) ([]any, error) {
	tx, err := connection.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if engine == types.Engine_POSTGRES && qCtx.UsePostgresDatabaseOwner {
		const query = `
		SELECT
			u.rolname
		FROM
			pg_roles AS u JOIN pg_database AS d ON (d.datdba = u.oid)
		WHERE
			d.datname = current_database();
		`
		var owner string
		if err := tx.QueryRowContext(ctx, query).Scan(&owner); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET ROLE '%s';", owner)); err != nil {
			return nil, err
		}
	}

	for _, preExec := range qCtx.PreExecutions {
		if preExec != "" {
			if _, err := tx.ExecContext(ctx, preExec); err != nil {
				return nil, errors.Wrapf(err, "failed to execute pre-execution: %s", preExec)
			}
		}
	}

	rows, err := tx.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	colCount := len(columnTypes)

	var columnTypeNames []string
	for _, v := range columnTypes {
		// DatabaseTypeName returns the database system name of the column type.
		// refer: https://pkg.go.dev/database/sql#ColumnType.DatabaseTypeName
		columnTypeNames = append(columnTypeNames, strings.ToUpper(v.DatabaseTypeName()))
	}

	data := []any{}
	for rows.Next() {
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
		return nil, err
	}

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
