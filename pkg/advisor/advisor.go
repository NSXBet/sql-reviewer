package advisor

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	"github.com/pkg/errors"

	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// SQLReviewRuleType is the type of schema rule.
type SQLReviewRuleType string

const (
	// SchemaRuleMySQLEngine require InnoDB as the storage engine.
	SchemaRuleMySQLEngine SQLReviewRuleType = "engine.mysql.use-innodb"

	// SchemaRuleFullyQualifiedObjectName enforces using fully qualified object name.
	SchemaRuleFullyQualifiedObjectName SQLReviewRuleType = "naming.fully-qualified"
	// SchemaRuleTableNaming enforce the table name format.
	SchemaRuleTableNaming SQLReviewRuleType = "naming.table"
	// SchemaRuleColumnNaming enforce the column name format.
	SchemaRuleColumnNaming SQLReviewRuleType = "naming.column"
	// SchemaRulePKNaming enforce the primary key name format.
	SchemaRulePKNaming SQLReviewRuleType = "naming.index.pk"
	// SchemaRuleUKNaming enforce the unique key name format.
	SchemaRuleUKNaming SQLReviewRuleType = "naming.index.uk"
	// SchemaRuleFKNaming enforce the foreign key name format.
	SchemaRuleFKNaming SQLReviewRuleType = "naming.index.fk"
	// SchemaRuleIDXNaming enforce the index name format.
	SchemaRuleIDXNaming SQLReviewRuleType = "naming.index.idx"
	// SchemaRuleAutoIncrementColumnNaming enforce the auto_increment column name format.
	SchemaRuleAutoIncrementColumnNaming SQLReviewRuleType = "naming.column.auto-increment"
	// SchemaRuleTableNameNoKeyword enforce the table name not to use keyword.
	SchemaRuleTableNameNoKeyword SQLReviewRuleType = "naming.table.no-keyword"
	// SchemaRuleIdentifierNoKeyword enforce the identifier not to use keyword.
	SchemaRuleIdentifierNoKeyword SQLReviewRuleType = "naming.identifier.no-keyword"
	// SchemaRuleIdentifierCase enforce the identifier case.
	SchemaRuleIdentifierCase SQLReviewRuleType = "naming.identifier.case"

	// SchemaRuleStatementNoSelectAll disallow 'SELECT *'.
	SchemaRuleStatementNoSelectAll SQLReviewRuleType = "statement.select.no-select-all"
	// SchemaRuleStatementRequireWhereForSelect require 'WHERE' clause for SELECT statements.
	SchemaRuleStatementRequireWhereForSelect SQLReviewRuleType = "statement.where.require.select"
	// SchemaRuleStatementRequireWhereForUpdateDelete require 'WHERE' clause for UPDATE and DELETE statements.
	SchemaRuleStatementRequireWhereForUpdateDelete SQLReviewRuleType = "statement.where.require.update-delete"
	// SchemaRuleStatementNoLeadingWildcardLike disallow leading '%' in LIKE, e.g. LIKE foo = '%x' is not allowed.
	SchemaRuleStatementNoLeadingWildcardLike SQLReviewRuleType = "statement.where.no-leading-wildcard-like"
	// SchemaRuleStatementDisallowOnDelCascade disallows ON DELETE CASCADE clauses.
	SchemaRuleStatementDisallowOnDelCascade SQLReviewRuleType = "statement.disallow-on-del-cascade"
	// SchemaRuleStatementDisallowRemoveTblCascade disallows CASCADE when removing a table.
	SchemaRuleStatementDisallowRemoveTblCascade SQLReviewRuleType = "statement.disallow-rm-tbl-cascade"
	// SchemaRuleStatementDisallowCommit disallow using commit in the issue.
	SchemaRuleStatementDisallowCommit SQLReviewRuleType = "statement.disallow-commit"
	// SchemaRuleStatementDisallowLimit disallow the LIMIT clause in INSERT, DELETE and UPDATE statements.
	SchemaRuleStatementDisallowLimit SQLReviewRuleType = "statement.disallow-limit"
	// SchemaRuleStatementDisallowOrderBy disallow the ORDER BY clause in DELETE and UPDATE statements.
	SchemaRuleStatementDisallowOrderBy SQLReviewRuleType = "statement.disallow-order-by"
	// SchemaRuleStatementMergeAlterTable disallow redundant ALTER TABLE statements.
	SchemaRuleStatementMergeAlterTable SQLReviewRuleType = "statement.merge-alter-table"
	// SchemaRuleStatementInsertRowLimit enforce the insert row limit.
	SchemaRuleStatementInsertRowLimit SQLReviewRuleType = "statement.insert.row-limit"
	// SchemaRuleStatementInsertMustSpecifyColumn enforce the insert column specified.
	SchemaRuleStatementInsertMustSpecifyColumn SQLReviewRuleType = "statement.insert.must-specify-column"
	// SchemaRuleStatementInsertDisallowOrderByRand disallow the order by rand in the INSERT statement.
	SchemaRuleStatementInsertDisallowOrderByRand SQLReviewRuleType = "statement.insert.disallow-order-by-rand"
	// SchemaRuleStatementAffectedRowLimit enforce the UPDATE/DELETE affected row limit.
	SchemaRuleStatementAffectedRowLimit SQLReviewRuleType = "statement.affected-row-limit"
	// SchemaRuleStatementDMLDryRun dry run the dml.
	SchemaRuleStatementDMLDryRun SQLReviewRuleType = "statement.dml-dry-run"
	// SchemaRuleStatementDisallowAddColumnWithDefault disallow to add column with DEFAULT.
	SchemaRuleStatementDisallowAddColumnWithDefault = "statement.disallow-add-column-with-default"
	// SchemaRuleStatementAddCheckNotValid require add check constraints not valid.
	SchemaRuleStatementAddCheckNotValid = "statement.add-check-not-valid"
	// SchemaRuleStatementAddFKNotValid require add foreign key not valid.
	SchemaRuleStatementAddFKNotValid = "statement.add-foreign-key-not-valid"
	// SchemaRuleStatementDisallowAddNotNull disallow to add NOT NULL.
	SchemaRuleStatementDisallowAddNotNull = "statement.disallow-add-not-null"
	// SchemaRuleStatementDisallowAddColumn disallow to add column.
	SchemaRuleStatementSelectFullTableScan = "statement.select-full-table-scan"
	// SchemaRuleStatementCreateSpecifySchema disallow to create table without specifying schema.
	SchemaRuleStatementCreateSpecifySchema = "statement.create-specify-schema"
	// SchemaRuleStatementCheckSetRoleVariable require add a check for SET ROLE variable.
	SchemaRuleStatementCheckSetRoleVariable = "statement.check-set-role-variable"
	// SchemaRuleStatementDisallowUsingFilesort disallow using filesort in execution plan.
	SchemaRuleStatementDisallowUsingFilesort = "statement.disallow-using-filesort"
	// SchemaRuleStatementDisallowUsingTemporary disallow using temporary in execution plan.
	SchemaRuleStatementDisallowUsingTemporary = "statement.disallow-using-temporary"
	// SchemaRuleStatementWhereNoEqualNull check the WHERE clause no equal null.
	SchemaRuleStatementWhereNoEqualNull = "statement.where.no-equal-null"
	// SchemaRuleStatementWhereDisallowFunctionsAndCaculations disallow using function in WHERE clause.
	SchemaRuleStatementWhereDisallowFunctionsAndCaculations = "statement.where.disallow-functions-and-calculations"
	// SchemaRuleStatementQueryMinumumPlanLevel enforce the minimum plan level.
	SchemaRuleStatementQueryMinumumPlanLevel = "statement.query.minimum-plan-level"
	// SchemaRuleStatementWhereMaximumLogicalOperatorCount enforce the maximum logical operator count in WHERE clause.
	SchemaRuleStatementWhereMaximumLogicalOperatorCount = "statement.where.maximum-logical-operator-count"
	// SchemaRuleStatementMaximumLimitValue enforce the maximum limit value.
	SchemaRuleStatementMaximumLimitValue = "statement.maximum-limit-value"
	// SchemaRuleStatementMaximumJoinTableCount enforce the maximum join table count in the statement.
	SchemaRuleStatementMaximumJoinTableCount = "statement.maximum-join-table-count"
	// SchemaRuleStatementMaximumStatementsInTransaction enforce the maximum statements in transaction.
	SchemaRuleStatementMaximumStatementsInTransaction = "statement.maximum-statements-in-transaction"
	// SchemaRuleStatementJoinStrictColumnAttrs enforce the join strict column attributes.
	SchemaRuleStatementJoinStrictColumnAttrs = "statement.join-strict-column-attrs"
	// SchemaRuleStatementDisallowMixInDDL disallows DML statements in DDL statements.
	SchemaRuleStatementDisallowMixInDDL = "statement.disallow-mix-in-ddl"
	// SchemaRuleStatementDisallowMixInDML disallows DDL statements in DML statements.
	SchemaRuleStatementDisallowMixInDML = "statement.disallow-mix-in-dml"
	// SchemaRuleStatementPriorBackupCheck checks for prior backup.
	SchemaRuleStatementPriorBackupCheck = "statement.prior-backup-check"
	// SchemaRuleStatementNonTransactional checks for non-transactional statements.
	SchemaRuleStatementNonTransactional = "statement.non-transactional"
	// SchemaRuleStatementAddColumnWithoutPosition check no position in ADD COLUMN clause.
	SchemaRuleStatementAddColumnWithoutPosition = "statement.add-column-without-position"
	// SchemaRuleStatementDisallowOfflineDDL disallow offline ddl.
	SchemaRuleStatementDisallowOfflineDDL = "statement.disallow-offline-ddl"
	// SchemaRuleStatementDisallowCrossDBQueries disallow cross database queries.
	SchemaRuleStatementDisallowCrossDBQueries = "statement.disallow-cross-db-queries"
	// SchemaRuleStatementMaxExecutionTime enforce the maximum execution time.
	SchemaRuleStatementMaxExecutionTime = "statement.max-execution-time"
	// SchemaRuleStatementRequireAlgorithmOption require set ALGORITHM option in ALTER TABLE statement.
	SchemaRuleStatementRequireAlgorithmOption = "statement.require-algorithm-option"
	// SchemaRuleStatementRequireLockOption require set LOCK option in ALTER TABLE statement.
	SchemaRuleStatementRequireLockOption = "statement.require-lock-option"
	// SchemaRuleStatementObjectOwnerCheck checks the object owner for the statement.
	SchemaRuleStatementObjectOwnerCheck = "statement.object-owner-check"
	// SchemaRuleTableRequirePK require the table to have a primary key.
	SchemaRuleTableRequirePK SQLReviewRuleType = "table.require-pk"
	// SchemaRuleTableNoFK require the table disallow the foreign key.
	SchemaRuleTableNoFK SQLReviewRuleType = "table.no-foreign-key"
	// SchemaRuleTableDropNamingConvention require only the table following the naming convention can be deleted.
	SchemaRuleTableDropNamingConvention SQLReviewRuleType = "table.drop-naming-convention"
	// SchemaRuleTableCommentConvention enforce the table comment convention.
	SchemaRuleTableCommentConvention SQLReviewRuleType = "table.comment"
	// SchemaRuleTableDisallowPartition disallow the table partition.
	SchemaRuleTableDisallowPartition SQLReviewRuleType = "table.disallow-partition"
	// SchemaRuleTableDisallowTrigger disallow the table trigger.
	SchemaRuleTableDisallowTrigger SQLReviewRuleType = "table.disallow-trigger"
	// SchemaRuleTableNoDuplicateIndex require the table no duplicate index.
	SchemaRuleTableNoDuplicateIndex SQLReviewRuleType = "table.no-duplicate-index"
	// SchemaRuleTableTextFieldsTotalLength enforce the total length of text fields.
	SchemaRuleTableTextFieldsTotalLength SQLReviewRuleType = "table.text-fields-total-length"
	// SchemaRuleTableDisallowSetCharset disallow set table charset.
	SchemaRuleTableDisallowSetCharset SQLReviewRuleType = "table.disallow-set-charset"
	// SchemaRuleTableDisallowDDL disallow executing DDL for specific tables.
	SchemaRuleTableDisallowDDL SQLReviewRuleType = "table.disallow-ddl"
	// SchemaRuleTableDisallowDML disallow executing DML on specific tables.
	SchemaRuleTableDisallowDML SQLReviewRuleType = "table.disallow-dml"
	// SchemaRuleTableLimitSize  restrict access to tables based on size.
	SchemaRuleTableLimitSize SQLReviewRuleType = "table.limit-size"
	// SchemaRuleTableRequireCharset enforce the table charset.
	SchemaRuleTableRequireCharset SQLReviewRuleType = "table.require-charset"
	// SchemaRuleTableRequireCollation enforce the table collation.
	SchemaRuleTableRequireCollation SQLReviewRuleType = "table.require-collation"
	// SchemaRuleRequiredColumn enforce the required columns in each table.
	SchemaRuleRequiredColumn SQLReviewRuleType = "column.required"
	// SchemaRuleColumnNotNull enforce the columns cannot have NULL value.
	SchemaRuleColumnNotNull SQLReviewRuleType = "column.no-null"
	// SchemaRuleColumnDisallowChangeType disallow change column type.
	SchemaRuleColumnDisallowChangeType SQLReviewRuleType = "column.disallow-change-type"
	// SchemaRuleColumnSetDefaultForNotNull require the not null column to set default value.
	SchemaRuleColumnSetDefaultForNotNull SQLReviewRuleType = "column.set-default-for-not-null"
	// SchemaRuleColumnDisallowChange disallow CHANGE COLUMN statement.
	SchemaRuleColumnDisallowChange SQLReviewRuleType = "column.disallow-change"
	// SchemaRuleColumnDisallowChangingOrder disallow changing column order.
	SchemaRuleColumnDisallowChangingOrder SQLReviewRuleType = "column.disallow-changing-order"
	// SchemaRuleColumnDisallowDrop disallow drop column.
	SchemaRuleColumnDisallowDrop SQLReviewRuleType = "column.disallow-drop"
	// SchemaRuleColumnDisallowDropInIndex disallow drop index column.
	SchemaRuleColumnDisallowDropInIndex SQLReviewRuleType = "column.disallow-drop-in-index"
	// SchemaRuleColumnCommentConvention enforce the column comment convention.
	SchemaRuleColumnCommentConvention SQLReviewRuleType = "column.comment"
	// SchemaRuleColumnAutoIncrementMustInteger require the auto-increment column to be integer.
	SchemaRuleColumnAutoIncrementMustInteger SQLReviewRuleType = "column.auto-increment-must-integer"
	// SchemaRuleColumnTypeDisallowList enforce the column type disallow list.
	SchemaRuleColumnTypeDisallowList SQLReviewRuleType = "column.type-disallow-list"
	// SchemaRuleColumnDisallowSetCharset disallow set column charset.
	SchemaRuleColumnDisallowSetCharset SQLReviewRuleType = "column.disallow-set-charset"
	// SchemaRuleColumnMaximumCharacterLength enforce the maximum character length.
	SchemaRuleColumnMaximumCharacterLength SQLReviewRuleType = "column.maximum-character-length"
	// SchemaRuleColumnMaximumVarcharLength enforce the maximum varchar length.
	SchemaRuleColumnMaximumVarcharLength SQLReviewRuleType = "column.maximum-varchar-length"
	// SchemaRuleColumnAutoIncrementInitialValue enforce the initial auto-increment value.
	SchemaRuleColumnAutoIncrementInitialValue SQLReviewRuleType = "column.auto-increment-initial-value"
	// SchemaRuleColumnAutoIncrementMustUnsigned enforce the auto-increment column to be unsigned.
	SchemaRuleColumnAutoIncrementMustUnsigned SQLReviewRuleType = "column.auto-increment-must-unsigned"
	// SchemaRuleCurrentTimeColumnCountLimit enforce the current column count limit.
	SchemaRuleCurrentTimeColumnCountLimit SQLReviewRuleType = "column.current-time-count-limit"
	// SchemaRuleColumnRequireDefault enforce the column default.
	SchemaRuleColumnRequireDefault SQLReviewRuleType = "column.require-default"
	// SchemaRuleColumnDefaultDisallowVolatile enforce the column default disallow volatile.
	SchemaRuleColumnDefaultDisallowVolatile SQLReviewRuleType = "column.default-disallow-volatile"
	// SchemaRuleAddNotNullColumnRequireDefault enforce the adding not null column requires default.
	SchemaRuleAddNotNullColumnRequireDefault SQLReviewRuleType = "column.add-not-null-require-default"
	// SchemaRuleColumnRequireCharset enforce the column require charset.
	SchemaRuleColumnRequireCharset SQLReviewRuleType = "column.require-charset"
	// SchemaRuleColumnRequireCollation enforce the column require collation.
	SchemaRuleColumnRequireCollation SQLReviewRuleType = "column.require-collation"

	// SchemaRuleSchemaBackwardCompatibility enforce the MySQL and TiDB support check whether the schema change is backward compatible.
	SchemaRuleSchemaBackwardCompatibility SQLReviewRuleType = "schema.backward-compatibility"

	// SchemaRuleDropEmptyDatabase enforce the MySQL and TiDB support check if the database is empty before users drop it.
	SchemaRuleDropEmptyDatabase SQLReviewRuleType = "database.drop-empty-database"

	// SchemaRuleIndexNoDuplicateColumn require the index no duplicate column.
	SchemaRuleIndexNoDuplicateColumn SQLReviewRuleType = "index.no-duplicate-column"
	// SchemaRuleIndexKeyNumberLimit enforce the index key number limit.
	SchemaRuleIndexKeyNumberLimit SQLReviewRuleType = "index.key-number-limit"
	// SchemaRuleIndexPKTypeLimit enforce the type restriction of columns in primary key.
	SchemaRuleIndexPKTypeLimit SQLReviewRuleType = "index.pk-type-limit"
	// SchemaRuleIndexTypeNoBlob enforce the type restriction of columns in index.
	SchemaRuleIndexTypeNoBlob SQLReviewRuleType = "index.type-no-blob"
	// SchemaRuleIndexTotalNumberLimit enforce the index total number limit.
	SchemaRuleIndexTotalNumberLimit SQLReviewRuleType = "index.total-number-limit"
	// SchemaRuleIndexPrimaryKeyTypeAllowlist enforce the primary key type allowlist.
	SchemaRuleIndexPrimaryKeyTypeAllowlist SQLReviewRuleType = "index.primary-key-type-allowlist"
	// SchemaRuleCreateIndexConcurrently require creating indexes concurrently.
	SchemaRuleCreateIndexConcurrently SQLReviewRuleType = "index.create-concurrently"
	// SchemaRuleIndexTypeAllowList enforce the index type allowlist.
	SchemaRuleIndexTypeAllowList SQLReviewRuleType = "index.type-allow-list"
	// SchemaRuleIndexNotRedundant prohibits createing redundant indices.
	SchemaRuleIndexNotRedundant SQLReviewRuleType = "index.not-redundant"

	// SchemaRuleCharsetAllowlist enforce the charset allowlist.
	SchemaRuleCharsetAllowlist SQLReviewRuleType = "system.charset.allowlist"
	// SchemaRuleCollationAllowlist enforce the collation allowlist.
	SchemaRuleCollationAllowlist SQLReviewRuleType = "system.collation.allowlist"
	// SchemaRuleCommentLength limit comment length.
	SchemaRuleCommentLength SQLReviewRuleType = "system.comment.length"
	// SchemaRuleProcedureDisallowCreate disallow create procedure.
	SchemaRuleProcedureDisallowCreate SQLReviewRuleType = "system.procedure.disallow-create"
	// SchemaRuleEventDisallowCreate disallow create event.
	SchemaRuleEventDisallowCreate SQLReviewRuleType = "system.event.disallow-create"
	// SchemaRuleViewDisallowCreate disallow create view.
	SchemaRuleViewDisallowCreate SQLReviewRuleType = "system.view.disallow-create"
	// SchemaRuleFunctionDisallowCreate disallow create function.
	SchemaRuleFunctionDisallowCreate SQLReviewRuleType = "system.function.disallow-create"
	// SchemaRuleFunctionDisallowList enforce the disallowed function list.
	SchemaRuleFunctionDisallowList SQLReviewRuleType = "system.function.disallowed-list"

	// SchemaRuleOnlineMigration advises using online migration to migrate large tables.
	SchemaRuleOnlineMigration SQLReviewRuleType = "advice.online-migration"

	// TableNameTemplateToken is the token for table name.
	TableNameTemplateToken = "{{table}}"
	// ColumnListTemplateToken is the token for column name list.
	ColumnListTemplateToken = "{{column_list}}"
	// ReferencingTableNameTemplateToken is the token for referencing table name.
	ReferencingTableNameTemplateToken = "{{referencing_table}}"
	// ReferencingColumnNameTemplateToken is the token for referencing column name.
	ReferencingColumnNameTemplateToken = "{{referencing_column}}"
	// ReferencedTableNameTemplateToken is the token for referenced table name.
	ReferencedTableNameTemplateToken = "{{referenced_table}}"
	// ReferencedColumnNameTemplateToken is the token for referenced column name.
	ReferencedColumnNameTemplateToken = "{{referenced_column}}"

	// defaultNameLengthLimit is the default length limit for naming rules.
	// PostgreSQL has it's own naming length limit, will auto slice the name to make sure its length <= 63
	// https://www.postgresql.org/docs/current/limits.html.
	// While MySQL does not enforce the limit, thus we use PostgreSQL's 63 as the default limit.
	defaultNameLengthLimit = 63
)

const (
	// SyntaxErrorTitle is the error title for syntax error.
	SyntaxErrorTitle string = "Syntax error"
)

// Type is the type of advisor.
type Type string

// SQLReviewCheckContext is an alias for Context for backward compatibility.
type SQLReviewCheckContext = Context

// catalogInterface is the interface for catalog.
type catalogInterface interface {
	GetFinder() *catalog.Finder
}

// NewStatusBySQLReviewRuleLevel returns status by SQLReviewRuleLevel.
func NewStatusBySQLReviewRuleLevel(level types.SQLReviewRuleLevel) (types.Advice_Status, error) {
	switch level {
	case types.SQLReviewRuleLevel_ERROR:
		return types.Advice_ERROR, nil
	case types.SQLReviewRuleLevel_WARNING:
		return types.Advice_WARNING, nil
	}
	return types.Advice_STATUS_UNSPECIFIED, errors.Errorf("unexpected rule level type: %v", level)
}

// Context is the unified context for SQL review check and advisor.
type Context struct {
	// Common fields
	ChangeType            types.PlanCheckRunConfig_ChangeDatabaseType
	DBSchema              *types.DatabaseSchemaMetadata
	EnablePriorBackup     bool
	ClassificationConfig  *types.DataClassificationSetting_DataClassificationConfig
	ListDatabaseNamesFunc func(ctx context.Context, instanceID string) ([]string, error)
	InstanceID            string
	IsObjectCaseSensitive bool
	CurrentDatabase       string
	UsePostgresDatabaseOwner bool

	// Database-specific fields
	Charset    string
	Collation  string
	DBType     types.Engine

	// Catalog and driver  
	Catalog catalogInterface
	Driver  *sql.DB

	// SQL review rule special fields
	AST        any
	Rule       *types.SQLReviewRule
	Statements string

	// Used for test only
	NoAppendBuiltin bool
}

// Advisor is the interface for advisor.
type Advisor interface {
	Check(ctx context.Context, checkCtx Context) ([]*types.Advice, error)
}

var (
	advisorMu sync.RWMutex
	advisors  = make(map[types.Engine]map[Type]Advisor)
)

// Register makes an advisor available by the provided id.
// If Register is called twice with the same name or if advisor is nil,
// it panics.
func Register(dbType types.Engine, advType Type, f Advisor) {
	advisorMu.Lock()
	defer advisorMu.Unlock()
	if f == nil {
		panic("advisor: Register advisor is nil")
	}
	dbAdvisors, ok := advisors[dbType]
	if !ok {
		advisors[dbType] = map[Type]Advisor{
			advType: f,
		}
	} else {
		if _, dup := dbAdvisors[advType]; dup {
			panic(fmt.Sprintf("advisor: Register called twice for advisor %v for %v", advType, dbType))
		}
		dbAdvisors[advType] = f
	}
}

// Check runs the advisor and returns the advices.
func Check(ctx context.Context, dbType types.Engine, advType Type, checkCtx Context) (adviceList []*types.Advice, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			panicErr, ok := panicErr.(error)
			if !ok {
				panicErr = errors.Errorf("%v", panicErr)
			}
			err = errors.Errorf("advisor check PANIC RECOVER, type: %v, err: %v", advType, panicErr)
			slog.Error("advisor check PANIC RECOVER", "error", panicErr)
		}
	}()

	advisorMu.RLock()
	dbAdvisors, ok := advisors[dbType]
	defer advisorMu.RUnlock()
	if !ok {
		return nil, errors.Errorf("advisor: unknown db advisor type %v", dbType)
	}

	f, ok := dbAdvisors[advType]
	if !ok {
		return nil, errors.Errorf("advisor: unknown advisor %v for %v", advType, dbType)
	}

	return f.Check(ctx, checkCtx)
}
