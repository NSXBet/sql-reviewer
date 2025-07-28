package catalog

import (
	"fmt"

	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// WalkThroughErrorType is the type of WalkThroughError.
type WalkThroughErrorType int

const (
	// PrimaryKeyName is the string for PK.
	PrimaryKeyName string = "PRIMARY"
	// FullTextName is the string for FULLTEXT.
	FullTextName string = "FULLTEXT"
	// SpatialName is the string for SPATIAL.
	SpatialName string = "SPATIAL"

	// ErrorTypeUnsupported is the error for unsupported cases.
	ErrorTypeUnsupported WalkThroughErrorType = 1
	// ErrorTypeInternal is the error for internal errors.
	ErrorTypeInternal WalkThroughErrorType = 2
	// ErrorTypeInvalidStatement is the error type for invalid statement errors.
	ErrorTypeInvalidStatement WalkThroughErrorType = 3

	// 101 parse error type.

	// ErrorTypeParseError is the error in parsing.
	ErrorTypeParseError WalkThroughErrorType = 101
	// ErrorTypeDeparseError is the error in deparsing.
	ErrorTypeDeparseError WalkThroughErrorType = 102
	// ErrorTypeSetLineError is the error in setting line for statement.
	ErrorTypeSetLineError WalkThroughErrorType = 103

	// 201 ~ 299 database error type.

	// ErrorTypeAccessOtherDatabase is the error that try to access other database.
	ErrorTypeAccessOtherDatabase = 201
	// ErrorTypeDatabaseIsDeleted is the error that try to access the deleted database.
	ErrorTypeDatabaseIsDeleted = 202
	// ErrorTypeReferenceOtherDatabase is the error that try to reference other database.
	ErrorTypeReferenceOtherDatabase = 203

	// 301 ~ 399 table error type.

	// ErrorTypeTableExists is the error that table exists.
	ErrorTypeTableExists = 301
	// ErrorTypeTableNotExists is the error that table not exists.
	ErrorTypeTableNotExists = 302
	// ErrorTypeUseCreateTableAs is the error that using CREATE TABLE AS statements.
	ErrorTypeUseCreateTableAs = 303
	// ErrorTypeTableIsReferencedByView is the error that table is referenced by view.
	ErrorTypeTableIsReferencedByView = 304
	// ErrorTypeViewNotExists is the error that view not exists.
	ErrorTypeViewNotExists = 305
	// ErrorTypeViewExists is the error that view exists.
	ErrorTypeViewExists = 306

	// 401 ~ 499 column error type.

	// ErrorTypeColumnExists is the error that column exists.
	ErrorTypeColumnExists = 401
	// ErrorTypeColumnNotExists is the error that column not exists.
	ErrorTypeColumnNotExists = 402
	// ErrorTypeDropAllColumns is the error that dropping all columns in a table.
	ErrorTypeDropAllColumns = 403
	// ErrorTypeAutoIncrementExists is the error that auto_increment exists.
	ErrorTypeAutoIncrementExists = 404
	// ErrorTypeOnUpdateColumnNotDatetimeOrTimestamp is the error that the ON UPDATE column is not datetime or timestamp.
	ErrorTypeOnUpdateColumnNotDatetimeOrTimestamp = 405
	// ErrorTypeSetNullDefaultForNotNullColumn is the error that setting NULL default value for the NOT NULL column.
	ErrorTypeSetNullDefaultForNotNullColumn = 406
	// ErrorTypeInvalidColumnTypeForDefaultValue is the error that invalid column type for default value.
	ErrorTypeInvalidColumnTypeForDefaultValue = 407
	// ErrorTypeColumnIsReferencedByView is the error that column is referenced by view.
	ErrorTypeColumnIsReferencedByView = 408

	// 501 ~ 599 index error type.

	// ErrorTypePrimaryKeyExists is the error that PK exists.
	ErrorTypePrimaryKeyExists = 501
	// ErrorTypeIndexExists is the error that index exists.
	ErrorTypeIndexExists = 502
	// ErrorTypeIndexEmptyKeys is the error that index has empty keys.
	ErrorTypeIndexEmptyKeys = 503
	// ErrorTypePrimaryKeyNotExists is the error that PK does not exist.
	ErrorTypePrimaryKeyNotExists = 504
	// ErrorTypeIndexNotExists is the error that index does not exist.
	ErrorTypeIndexNotExists = 505
	// ErrorTypeIncorrectIndexName is the incorrect index name error.
	ErrorTypeIncorrectIndexName = 506
	// ErrorTypeSpatialIndexKeyNullable is the error that keys in spatial index are nullable.
	ErrorTypeSpatialIndexKeyNullable = 507

	// 601 ~ 699 insert statement error type.

	// ErrorTypeInsertColumnCountNotMatchValueCount is the error that column count doesn't match value count.
	ErrorTypeInsertColumnCountNotMatchValueCount = 601
	// ErrorTypeInsertSpecifiedColumnTwice is the error that column specified twice in INSERT.
	ErrorTypeInsertSpecifiedColumnTwice = 602
	// ErrorTypeInsertNullIntoNotNullColumn is the error that insert NULL into NOT NULL columns.
	ErrorTypeInsertNullIntoNotNullColumn = 603

	// 701 ~ 799 schema error type.

	// ErrorTypeSchemaNotExists is the error that schema does not exist.
	ErrorTypeSchemaNotExists = 701
	// ErrorTypeSchemaExists is the error that schema already exists.
	ErrorTypeSchemaExists = 702

	// 801 ~ 899 relation error type.

	// ErrorTypeRelationExists is the error that relation already exists.
	ErrorTypeRelationExists = 801

	// 901 ~ 999 constraint error type.

	// ErrorTypeConstraintNotExists is the error that constraint doesn't exist.
	ErrorTypeConstraintNotExists = 901
)

// WalkThroughError is the error for walking-through.
type WalkThroughError struct {
	Type    WalkThroughErrorType
	Content string
	// TODO(zp): position
	Line int

	Payload any
}

// NewRelationExistsError returns a new ErrorTypeRelationExists.
func NewRelationExistsError(relationName string, schemaName string) *WalkThroughError {
	return &WalkThroughError{
		Type:    ErrorTypeRelationExists,
		Content: fmt.Sprintf("Relation %q already exists in schema %q", relationName, schemaName),
	}
}

// NewColumnNotExistsError returns a new ErrorTypeColumnNotExists.
func NewColumnNotExistsError(tableName string, columnName string) *WalkThroughError {
	return &WalkThroughError{
		Type:    ErrorTypeColumnNotExists,
		Content: fmt.Sprintf("Column `%s` does not exist in table `%s`", columnName, tableName),
	}
}

// NewIndexNotExistsError returns a new ErrorTypeIndexNotExists.
func NewIndexNotExistsError(tableName string, indexName string) *WalkThroughError {
	return &WalkThroughError{
		Type:    ErrorTypeIndexNotExists,
		Content: fmt.Sprintf("Index `%s` does not exist in table `%s`", indexName, tableName),
	}
}

// NewIndexExistsError returns a new ErrorTypeIndexExists.
func NewIndexExistsError(tableName string, indexName string) *WalkThroughError {
	return &WalkThroughError{
		Type:    ErrorTypeIndexExists,
		Content: fmt.Sprintf("Index `%s` already exists in table `%s`", indexName, tableName),
	}
}

// NewAccessOtherDatabaseError returns a new ErrorTypeAccessOtherDatabase.
func NewAccessOtherDatabaseError(current string, target string) *WalkThroughError {
	return &WalkThroughError{
		Type:    ErrorTypeAccessOtherDatabase,
		Content: fmt.Sprintf("Database `%s` is not the current database `%s`", target, current),
	}
}

// NewTableNotExistsError returns a new ErrorTypeTableNotExists.
func NewTableNotExistsError(tableName string) *WalkThroughError {
	return &WalkThroughError{
		Type:    ErrorTypeTableNotExists,
		Content: fmt.Sprintf("Table `%s` does not exist", tableName),
	}
}

// NewTableExistsError returns a new ErrorTypeTableExists.
func NewTableExistsError(tableName string) *WalkThroughError {
	return &WalkThroughError{
		Type:    ErrorTypeTableExists,
		Content: fmt.Sprintf("Table `%s` already exists", tableName),
	}
}

// Error implements the error interface.
func (e *WalkThroughError) Error() string {
	return e.Content
}

// WalkThrough will collect the catalog schema in the databaseState as it walks through the stmt.
func (d *DatabaseState) WalkThrough(ast any) error {
	switch d.dbType {
	case types.Engine_MYSQL, types.Engine_MARIADB, types.Engine_OCEANBASE:
		err := d.mysqlWalkThrough(ast)
		return err
	default:
		return &WalkThroughError{
			Type:    ErrorTypeUnsupported,
			Content: fmt.Sprintf("Walk-through doesn't support engine type: %s", d.dbType),
		}
	}
}
