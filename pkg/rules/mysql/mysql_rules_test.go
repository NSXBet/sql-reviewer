package mysql

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// UnifiedTestCase represents a single test case from the YAML file matching Bytebase format
type TestCase struct {
	Statement  string          `yaml:"statement"`
	ChangeType int             `yaml:"changeType"`
	Want       []*types.Advice `yaml:"want,omitempty"`
}

// RuleAdvisor interface for all rule advisors (using payload from context)
type RuleAdvisor interface {
	Check(
		ctx context.Context,
		statements string,
		rule *types.SQLReviewRule,
		checkContext advisor.Context,
	) ([]*types.Advice, error)
}

// RuleMapping maps rule names to their advisor instances and titles
type RuleMapping struct {
	Advisor  RuleAdvisor
	RuleType advisor.SQLReviewRuleType // Official rule type constant
	Title    string
}

// yamlFileNameToRuleType converts YAML file names (with underscores) to official rule type constants
func yamlFileNameToRuleType(fileName string) advisor.SQLReviewRuleType {
	// Convert YAML file names to official rule type constants
	switch fileName {
	// Column rules
	case "column_no_null":
		return advisor.SchemaRuleColumnNotNull
	case "column_required":
		return advisor.SchemaRuleRequiredColumn
	case "column_comment":
		return advisor.SchemaRuleColumnCommentConvention
	case "column_auto_increment_must_integer":
		return advisor.SchemaRuleColumnAutoIncrementMustInteger
	case "column_auto_increment_must_unsigned":
		return advisor.SchemaRuleColumnAutoIncrementMustUnsigned
	case "column_auto_increment_initial_value":
		return advisor.SchemaRuleColumnAutoIncrementInitialValue
	case "column_disallow_change":
		return advisor.SchemaRuleColumnDisallowChange
	case "column_disallow_change_type":
		return advisor.SchemaRuleColumnDisallowChangeType
	case "column_disallow_changing_order":
		return advisor.SchemaRuleColumnDisallowChangingOrder
	case "column_disallow_drop":
		return advisor.SchemaRuleColumnDisallowDrop
	case "column_disallow_drop_in_index":
		return advisor.SchemaRuleColumnDisallowDropInIndex
	case "column_disallow_set_charset":
		return advisor.SchemaRuleColumnDisallowSetCharset
	case "column_maximum_character_length":
		return advisor.SchemaRuleColumnMaximumCharacterLength
	case "column_maximum_varchar_length":
		return advisor.SchemaRuleColumnMaximumVarcharLength
	case "column_require_default":
		return advisor.SchemaRuleColumnRequireDefault
	case "column_set_default_for_not_null":
		return advisor.SchemaRuleColumnSetDefaultForNotNull
	case "column_type_disallow_list":
		return advisor.SchemaRuleColumnTypeDisallowList
	case "column_current_time_count_limit":
		return advisor.SchemaRuleCurrentTimeColumnCountLimit
	case "column_require_charset":
		return advisor.SchemaRuleColumnRequireCharset
	case "column_require_collation":
		return advisor.SchemaRuleColumnRequireCollation

	// Statement rules
	case "statement_select_no_select_all":
		return advisor.SchemaRuleStatementNoSelectAll
	case "statement_where_require_select":
		return advisor.SchemaRuleStatementRequireWhereForSelect
	case "statement_where_require_update_delete":
		return advisor.SchemaRuleStatementRequireWhereForUpdateDelete
	case "statement_where_no_equal_null":
		return advisor.SchemaRuleStatementWhereNoEqualNull
	case "statement_where_no_leading_wildcard_like":
		return advisor.SchemaRuleStatementNoLeadingWildcardLike
	case "statement_where_disallow_functions_and_calculations":
		return advisor.SchemaRuleStatementWhereDisallowFunctionsAndCaculations
	case "statement_where_maximum_logical_operator_count":
		return advisor.SchemaRuleStatementWhereMaximumLogicalOperatorCount
	case "statement_disallow_commit":
		return advisor.SchemaRuleStatementDisallowCommit
	case "statement_disallow_limit":
		return advisor.SchemaRuleStatementDisallowLimit
	case "statement_disallow_order_by":
		return advisor.SchemaRuleStatementDisallowOrderBy
	case "statement_disallow_mix_in_ddl":
		return advisor.SchemaRuleStatementDisallowMixInDDL
	case "statement_disallow_mix_in_dml":
		return advisor.SchemaRuleStatementDisallowMixInDML
	case "statement_insert_must_specify_column":
		return advisor.SchemaRuleStatementInsertMustSpecifyColumn
	case "statement_insert_disallow_order_by_rand":
		return advisor.SchemaRuleStatementInsertDisallowOrderByRand
	case "statement_insert_row_limit":
		return advisor.SchemaRuleStatementInsertRowLimit
	case "statement_add_column_without_position":
		return advisor.SchemaRuleStatementAddColumnWithoutPosition
	case "statement_affected_row_limit":
		return advisor.SchemaRuleStatementAffectedRowLimit
	case "statement_dml_dry_run":
		return advisor.SchemaRuleStatementDMLDryRun
	case "statement_maximum_limit_value":
		return advisor.SchemaRuleStatementMaximumLimitValue
	case "statement_maximum_join_table_count":
		return advisor.SchemaRuleStatementMaximumJoinTableCount
	case "statement_maximum_statements_in_transaction":
		return advisor.SchemaRuleStatementMaximumStatementsInTransaction
	case "statement_query_minimum_plan_level":
		return advisor.SchemaRuleStatementQueryMinumumPlanLevel
	case "statement_disallow_using_filesort":
		return advisor.SchemaRuleStatementDisallowUsingFilesort
	case "statement_disallow_using_temporary":
		return advisor.SchemaRuleStatementDisallowUsingTemporary
	case "statement_join_strict_column_attrs":
		return advisor.SchemaRuleStatementJoinStrictColumnAttrs
	case "statement_select_full_table_scan":
		return advisor.SchemaRuleStatementSelectFullTableScan
	case "statement_max_execution_time":
		return advisor.SchemaRuleStatementMaxExecutionTime
	case "statement_merge_alter_table":
		return advisor.SchemaRuleStatementMergeAlterTable
	case "statement_require_algorithm_option":
		return advisor.SchemaRuleStatementRequireAlgorithmOption
	case "statement_require_lock_option":
		return advisor.SchemaRuleStatementRequireLockOption

	// Table rules
	case "table_require_pk":
		return advisor.SchemaRuleTableRequirePK
	case "table_no_foreign_key":
		return advisor.SchemaRuleTableNoFK
	case "table_drop_naming_convention":
		return advisor.SchemaRuleTableDropNamingConvention
	case "table_comment":
		return advisor.SchemaRuleTableCommentConvention
	case "table_disallow_partition":
		return advisor.SchemaRuleTableDisallowPartition
	case "table_disallow_trigger":
		return advisor.SchemaRuleTableDisallowTrigger
	case "table_disallow_set_charset":
		return advisor.SchemaRuleTableDisallowSetCharset
	case "table_require_charset":
		return advisor.SchemaRuleTableRequireCharset
	case "table_require_collation":
		return advisor.SchemaRuleTableRequireCollation
	case "table_disallow_ddl":
		return advisor.SchemaRuleTableDisallowDDL
	case "table_disallow_dml":
		return advisor.SchemaRuleTableDisallowDML
	case "table_text_fields_total_length":
		return advisor.SchemaRuleTableTextFieldsTotalLength
	case "table_limit_size":
		return advisor.SchemaRuleTableLimitSize
	case "table_no_duplicate_index":
		return advisor.SchemaRuleTableNoDuplicateIndex

	// System rules
	case "system_event_disallow_create":
		return advisor.SchemaRuleEventDisallowCreate
	case "system_function_disallow_create":
		return advisor.SchemaRuleFunctionDisallowCreate
	case "system_procedure_disallow_create":
		return advisor.SchemaRuleProcedureDisallowCreate
	case "system_view_disallow_create":
		return advisor.SchemaRuleViewDisallowCreate

	// Naming rules
	case "naming_table":
		return advisor.SchemaRuleTableNaming
	case "naming_column":
		return advisor.SchemaRuleColumnNaming
	case "naming_column_auto_increment":
		return advisor.SchemaRuleAutoIncrementColumnNaming
	case "naming_identifier_no_keyword":
		return advisor.SchemaRuleIdentifierNoKeyword
	case "naming_index_pk":
		return advisor.SchemaRulePKNaming
	case "naming_index_uk":
		return advisor.SchemaRuleUKNaming
	case "naming_index_fk":
		return advisor.SchemaRuleFKNaming
	case "naming_index_idx":
		return advisor.SchemaRuleIDXNaming

	// Index rules
	case "index_no_duplicate_column":
		return advisor.SchemaRuleIndexNoDuplicateColumn
	case "index_key_number_limit":
		return advisor.SchemaRuleIndexKeyNumberLimit
	case "index_pk_type_limit":
		return advisor.SchemaRuleIndexPKTypeLimit
	case "index_type_no_blob":
		return advisor.SchemaRuleIndexTypeNoBlob
	case "index_total_number_limit":
		return advisor.SchemaRuleIndexTotalNumberLimit
	case "index_primary_key_type_allowlist":
		return advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist
	case "index_type_allow_list":
		return advisor.SchemaRuleIndexTypeAllowList

	// System rules
	case "system_charset_allowlist":
		return advisor.SchemaRuleCharsetAllowlist
	case "system_collation_allowlist":
		return advisor.SchemaRuleCollationAllowlist
	case "system_function_disallowed_list":
		return advisor.SchemaRuleFunctionDisallowList

	// Database rules
	case "database_drop_empty_database":
		return advisor.SchemaRuleDropEmptyDatabase

	// Engine rules
	case "engine_mysql_use_innodb":
		return advisor.SchemaRuleMySQLEngine

	// Schema rules
	case "schema_backward_compatibility":
		return advisor.SchemaRuleSchemaBackwardCompatibility

	default:
		return advisor.SQLReviewRuleType(fileName) // Fallback to original name
	}
}

// GetRuleMappings returns all available rule mappings using official rule type constants
// Note: Only includes rules that have been updated to the new signature
func GetRuleMappings() map[string]RuleMapping {
	return map[string]RuleMapping{
		// Rules updated to new signature (context-based)
		"table_require_pk": {
			Advisor:  &TableRequirePKAdvisor{},
			RuleType: advisor.SchemaRuleTableRequirePK,
			Title:    string(advisor.SchemaRuleTableRequirePK),
		},
		"column_disallow_change_type": {
			Advisor:  &ColumnDisallowChangeTypeAdvisor{},
			RuleType: advisor.SchemaRuleColumnDisallowChangeType,
			Title:    string(advisor.SchemaRuleColumnDisallowChangeType),
		},
		"column_disallow_drop": {
			Advisor:  &ColumnDisallowDropAdvisor{},
			RuleType: advisor.SchemaRuleColumnDisallowDrop,
			Title:    string(advisor.SchemaRuleColumnDisallowDrop),
		},
		"column_no_null": {
			Advisor:  &ColumnNoNullAdvisor{},
			RuleType: advisor.SchemaRuleColumnNotNull,
			Title:    string(advisor.SchemaRuleColumnNotNull),
		},
		"statement_select_no_select_all": {
			Advisor:  &StatementSelectNoSelectAllAdvisor{},
			RuleType: advisor.SchemaRuleStatementNoSelectAll,
			Title:    string(advisor.SchemaRuleStatementNoSelectAll),
		},
		"statement_where_require_select": {
			Advisor:  &StatementWhereRequireSelectAdvisor{},
			RuleType: advisor.SchemaRuleStatementRequireWhereForSelect,
			Title:    string(advisor.SchemaRuleStatementRequireWhereForSelect),
		},
		"statement_where_require_update_delete": {
			Advisor:  &StatementWhereRequireUpdateDeleteAdvisor{},
			RuleType: advisor.SchemaRuleStatementRequireWhereForUpdateDelete,
			Title:    string(advisor.SchemaRuleStatementRequireWhereForUpdateDelete),
		},
		"column_comment": {
			Advisor:  &ColumnCommentAdvisor{},
			RuleType: advisor.SchemaRuleColumnCommentConvention,
			Title:    string(advisor.SchemaRuleColumnCommentConvention),
		},
		"column_require_default": {
			Advisor:  &ColumnRequireDefaultAdvisor{},
			RuleType: advisor.SchemaRuleColumnRequireDefault,
			Title:    string(advisor.SchemaRuleColumnRequireDefault),
		},
		"column_set_default_for_not_null": {
			Advisor:  &ColumnSetDefaultForNotNullAdvisor{},
			RuleType: advisor.SchemaRuleColumnSetDefaultForNotNull,
			Title:    string(advisor.SchemaRuleColumnSetDefaultForNotNull),
		},
		"naming_column": {
			Advisor:  &NamingColumnAdvisor{},
			RuleType: advisor.SchemaRuleColumnNaming,
			Title:    string(advisor.SchemaRuleColumnNaming),
		},
		"index_no_duplicate_column": {
			Advisor:  &IndexNoDuplicateColumnAdvisor{},
			RuleType: advisor.SchemaRuleIndexNoDuplicateColumn,
			Title:    string(advisor.SchemaRuleIndexNoDuplicateColumn),
		},
		"table_comment": {
			Advisor:  &TableCommentAdvisor{},
			RuleType: advisor.SchemaRuleTableCommentConvention,
			Title:    string(advisor.SchemaRuleTableCommentConvention),
		},
		"column_required": {
			Advisor:  &ColumnRequiredAdvisor{},
			RuleType: advisor.SchemaRuleRequiredColumn,
			Title:    string(advisor.SchemaRuleRequiredColumn),
		},
		"engine_mysql_use_innodb": {
			Advisor:  &EngineMySQLUseInnoDBAdvisor{},
			RuleType: advisor.SchemaRuleMySQLEngine,
			Title:    string(advisor.SchemaRuleMySQLEngine),
		},
		"column_auto_increment_must_integer": {
			Advisor:  &ColumnAutoIncrementMustIntegerAdvisor{},
			RuleType: advisor.SchemaRuleColumnAutoIncrementMustInteger,
			Title:    string(advisor.SchemaRuleColumnAutoIncrementMustInteger),
		},
		"column_auto_increment_must_unsigned": {
			Advisor:  &ColumnAutoIncrementMustUnsignedAdvisor{},
			RuleType: advisor.SchemaRuleColumnAutoIncrementMustUnsigned,
			Title:    string(advisor.SchemaRuleColumnAutoIncrementMustUnsigned),
		},
		"table_no_foreign_key": {
			Advisor:  &TableNoForeignKeyAdvisor{},
			RuleType: advisor.SchemaRuleTableNoFK,
			Title:    string(advisor.SchemaRuleTableNoFK),
		},
		"index_key_number_limit": {
			Advisor:  &IndexKeyNumberLimitAdvisor{},
			RuleType: advisor.SchemaRuleIndexKeyNumberLimit,
			Title:    string(advisor.SchemaRuleIndexKeyNumberLimit),
		},
		"column_auto_increment_initial_value": {
			Advisor:  &ColumnAutoIncrementInitialValueAdvisor{},
			RuleType: advisor.SchemaRuleColumnAutoIncrementInitialValue,
			Title:    string(advisor.SchemaRuleColumnAutoIncrementInitialValue),
		},
		"system_event_disallow_create": {
			Advisor:  &SystemEventDisallowCreateAdvisor{},
			RuleType: advisor.SchemaRuleEventDisallowCreate,
			Title:    string(advisor.SchemaRuleEventDisallowCreate),
		},
		"system_function_disallow_create": {
			Advisor:  &SystemFunctionDisallowCreateAdvisor{},
			RuleType: advisor.SchemaRuleFunctionDisallowCreate,
			Title:    string(advisor.SchemaRuleFunctionDisallowCreate),
		},
		"system_function_disallowed_list": {
			Advisor:  &SystemFunctionDisallowedListAdvisor{},
			RuleType: advisor.SchemaRuleFunctionDisallowList,
			Title:    string(advisor.SchemaRuleFunctionDisallowList),
		},
		"system_procedure_disallow_create": {
			Advisor:  &SystemProcedureDisallowCreateAdvisor{},
			RuleType: advisor.SchemaRuleProcedureDisallowCreate,
			Title:    string(advisor.SchemaRuleProcedureDisallowCreate),
		},
		"system_view_disallow_create": {
			Advisor:  &SystemViewDisallowCreateAdvisor{},
			RuleType: advisor.SchemaRuleViewDisallowCreate,
			Title:    string(advisor.SchemaRuleViewDisallowCreate),
		},
		"column_current_time_count_limit": {
			Advisor:  &ColumnCurrentTimeCountLimitAdvisor{},
			RuleType: advisor.SchemaRuleCurrentTimeColumnCountLimit,
			Title:    string(advisor.SchemaRuleCurrentTimeColumnCountLimit),
		},
		"column_disallow_change": {
			Advisor:  &ColumnDisallowChangeAdvisor{},
			RuleType: advisor.SchemaRuleColumnDisallowChange,
			Title:    string(advisor.SchemaRuleColumnDisallowChange),
		},
		"naming_table": {
			Advisor:  &NamingTableConventionAdvisor{},
			RuleType: advisor.SchemaRuleTableNaming,
			Title:    string(advisor.SchemaRuleTableNaming),
		},
		"column_disallow_changing_order": {
			Advisor:  &ColumnDisallowChangingOrderAdvisor{},
			RuleType: advisor.SchemaRuleColumnDisallowChangingOrder,
			Title:    string(advisor.SchemaRuleColumnDisallowChangingOrder),
		},
		"column_disallow_drop_in_index": {
			Advisor:  &ColumnDisallowDropInIndexAdvisor{},
			RuleType: advisor.SchemaRuleColumnDisallowDropInIndex,
			Title:    string(advisor.SchemaRuleColumnDisallowDropInIndex),
		},
		"column_disallow_set_charset": {
			Advisor:  &ColumnDisallowSetCharsetAdvisor{},
			RuleType: advisor.SchemaRuleColumnDisallowSetCharset,
			Title:    string(advisor.SchemaRuleColumnDisallowSetCharset),
		},
		"column_maximum_character_length": {
			Advisor:  &ColumnMaximumCharacterLengthAdvisor{},
			RuleType: advisor.SchemaRuleColumnMaximumCharacterLength,
			Title:    string(advisor.SchemaRuleColumnMaximumCharacterLength),
		},
		"column_maximum_varchar_length": {
			Advisor:  &ColumnMaximumVarcharLengthAdvisor{},
			RuleType: advisor.SchemaRuleColumnMaximumVarcharLength,
			Title:    string(advisor.SchemaRuleColumnMaximumVarcharLength),
		},
		"column_type_disallow_list": {
			Advisor:  &ColumnTypeDisallowListAdvisor{},
			RuleType: advisor.SchemaRuleColumnTypeDisallowList,
			Title:    string(advisor.SchemaRuleColumnTypeDisallowList),
		},
		"database_drop_empty_database": {
			Advisor:  &DatabaseDropEmptyDatabaseAdvisor{},
			RuleType: advisor.SchemaRuleDropEmptyDatabase,
			Title:    string(advisor.SchemaRuleDropEmptyDatabase),
		},
		"index_pk_type_limit": {
			Advisor:  &IndexPkTypeLimitAdvisor{},
			RuleType: advisor.SchemaRuleIndexPKTypeLimit,
			Title:    string(advisor.SchemaRuleIndexPKTypeLimit),
		},
		"index_primary_key_type_allowlist": {
			Advisor:  &IndexPrimaryKeyTypeAllowlistAdvisor{},
			RuleType: advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist,
			Title:    string(advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist),
		},
		"index_total_number_limit": {
			Advisor:  &IndexTotalNumberLimitAdvisor{},
			RuleType: advisor.SchemaRuleIndexTotalNumberLimit,
			Title:    string(advisor.SchemaRuleIndexTotalNumberLimit),
		},
		"index_type_allow_list": {
			Advisor:  &IndexTypeAllowListAdvisor{},
			RuleType: advisor.SchemaRuleIndexTypeAllowList,
			Title:    string(advisor.SchemaRuleIndexTypeAllowList),
		},
		"index_type_no_blob": {
			Advisor:  &IndexTypeNoBlobAdvisor{},
			RuleType: advisor.SchemaRuleIndexTypeNoBlob,
			Title:    string(advisor.SchemaRuleIndexTypeNoBlob),
		},
		"naming_column_auto_increment": {
			Advisor:  &NamingAutoIncrementColumnAdvisor{},
			RuleType: advisor.SchemaRuleAutoIncrementColumnNaming,
			Title:    string(advisor.SchemaRuleAutoIncrementColumnNaming),
		},
		"naming_identifier_no_keyword": {
			Advisor:  &NamingIdentifierNoKeywordAdvisor{},
			RuleType: advisor.SchemaRuleIdentifierNoKeyword,
			Title:    string(advisor.SchemaRuleIdentifierNoKeyword),
		},
		"naming_index_fk": {
			Advisor:  &NamingIndexFKAdvisor{},
			RuleType: advisor.SchemaRuleFKNaming,
			Title:    string(advisor.SchemaRuleFKNaming),
		},
		"naming_index_idx": {
			Advisor:  &NamingIndexIdxAdvisor{},
			RuleType: advisor.SchemaRuleIDXNaming,
			Title:    string(advisor.SchemaRuleIDXNaming),
		},
		"naming_index_uk": {
			Advisor:  &NamingIndexUKAdvisor{},
			RuleType: advisor.SchemaRuleUKNaming,
			Title:    string(advisor.SchemaRuleUKNaming),
		},
		"statement_where_no_equal_null": {
			Advisor:  &StatementWhereNoEqualNullAdvisor{},
			RuleType: advisor.SchemaRuleStatementWhereNoEqualNull,
			Title:    string(advisor.SchemaRuleStatementWhereNoEqualNull),
		},
		"statement_disallow_commit": {
			Advisor:  &StatementDisallowCommitAdvisor{},
			RuleType: advisor.SchemaRuleStatementDisallowCommit,
			Title:    string(advisor.SchemaRuleStatementDisallowCommit),
		},
		"statement_disallow_limit": {
			Advisor:  &StatementDisallowLimitAdvisor{},
			RuleType: advisor.SchemaRuleStatementDisallowLimit,
			Title:    string(advisor.SchemaRuleStatementDisallowLimit),
		},
		"statement_disallow_order_by": {
			Advisor:  &StatementDisallowOrderByAdvisor{},
			RuleType: advisor.SchemaRuleStatementDisallowOrderBy,
			Title:    string(advisor.SchemaRuleStatementDisallowOrderBy),
		},
		"statement_insert_must_specify_column": {
			Advisor:  &StatementInsertMustSpecifyColumnAdvisor{},
			RuleType: advisor.SchemaRuleStatementInsertMustSpecifyColumn,
			Title:    string(advisor.SchemaRuleStatementInsertMustSpecifyColumn),
		},
		"statement_insert_disallow_order_by_rand": {
			Advisor:  &StatementInsertDisallowOrderByRandAdvisor{},
			RuleType: advisor.SchemaRuleStatementInsertDisallowOrderByRand,
			Title:    string(advisor.SchemaRuleStatementInsertDisallowOrderByRand),
		},
		"statement_maximum_limit_value": {
			Advisor:  &StatementMaximumLimitValueAdvisor{},
			RuleType: advisor.SchemaRuleStatementMaximumLimitValue,
			Title:    string(advisor.SchemaRuleStatementMaximumLimitValue),
		},
		"statement_maximum_join_table_count": {
			Advisor:  &StatementMaximumJoinTableCountAdvisor{},
			RuleType: advisor.SchemaRuleStatementMaximumJoinTableCount,
			Title:    string(advisor.SchemaRuleStatementMaximumJoinTableCount),
		},
		"statement_maximum_statements_in_transaction": {
			Advisor:  &StatementMaximumStatementsInTransactionAdvisor{},
			RuleType: advisor.SchemaRuleStatementMaximumStatementsInTransaction,
			Title:    string(advisor.SchemaRuleStatementMaximumStatementsInTransaction),
		},
		"statement_query_minimum_plan_level": {
			Advisor:  &StatementQueryMinumumPlanLevelAdvisor{},
			RuleType: advisor.SchemaRuleStatementQueryMinumumPlanLevel,
			Title:    string(advisor.SchemaRuleStatementQueryMinumumPlanLevel),
		},
		"statement_disallow_using_filesort": {
			Advisor:  &StatementDisallowUsingFilesortAdvisor{},
			RuleType: advisor.SchemaRuleStatementDisallowUsingFilesort,
			Title:    string(advisor.SchemaRuleStatementDisallowUsingFilesort),
		},
		"statement_disallow_using_temporary": {
			Advisor:  &StatementDisallowUsingTemporaryAdvisor{},
			RuleType: advisor.SchemaRuleStatementDisallowUsingTemporary,
			Title:    string(advisor.SchemaRuleStatementDisallowUsingTemporary),
		},
		"statement_join_strict_column_attrs": {
			Advisor:  &StatementJoinStrictColumnAttrsAdvisor{},
			RuleType: advisor.SchemaRuleStatementJoinStrictColumnAttrs,
			Title:    string(advisor.SchemaRuleStatementJoinStrictColumnAttrs),
		},
		"statement_select_full_table_scan": {
			Advisor:  &StatementSelectFullTableScanAdvisor{},
			RuleType: advisor.SchemaRuleStatementSelectFullTableScan,
			Title:    string(advisor.SchemaRuleStatementSelectFullTableScan),
		},
		"statement_max_execution_time": {
			Advisor:  &StatementMaxExecutionTimeAdvisor{},
			RuleType: advisor.SchemaRuleStatementMaxExecutionTime,
			Title:    string(advisor.SchemaRuleStatementMaxExecutionTime),
		},
		"statement_where_disallow_functions_and_calculations": {
			Advisor:  &StatementWhereDisallowFunctionsAndCalculationsAdvisor{},
			RuleType: advisor.SchemaRuleStatementWhereDisallowFunctionsAndCaculations,
			Title:    string(advisor.SchemaRuleStatementWhereDisallowFunctionsAndCaculations),
		},
		"statement_merge_alter_table": {
			Advisor:  &StatementMergeAlterTableAdvisor{},
			RuleType: advisor.SchemaRuleStatementMergeAlterTable,
			Title:    string(advisor.SchemaRuleStatementMergeAlterTable),
		},
		"statement_disallow_mix_in_ddl": {
			Advisor:  &StatementDisallowMixInDDLAdvisor{},
			RuleType: advisor.SchemaRuleStatementDisallowMixInDDL,
			Title:    string(advisor.SchemaRuleStatementDisallowMixInDDL),
		},
		"statement_disallow_mix_in_dml": {
			Advisor:  &StatementDisallowMixInDMLAdvisor{},
			RuleType: advisor.SchemaRuleStatementDisallowMixInDML,
			Title:    string(advisor.SchemaRuleStatementDisallowMixInDML),
		},
		"statement_insert_row_limit": {
			Advisor:  &StatementInsertRowLimitAdvisor{},
			RuleType: advisor.SchemaRuleStatementInsertRowLimit,
			Title:    string(advisor.SchemaRuleStatementInsertRowLimit),
		},
		"statement_dml_dry_run": {
			Advisor:  &StatementDmlDryRunAdvisor{},
			RuleType: advisor.SchemaRuleStatementDMLDryRun,
			Title:    string(advisor.SchemaRuleStatementDMLDryRun),
		},
		"table_no_duplicate_index": {
			Advisor:  &TableNoDuplicateIndexAdvisor{},
			RuleType: advisor.SchemaRuleTableNoDuplicateIndex,
			Title:    string(advisor.SchemaRuleTableNoDuplicateIndex),
		},
		"schema_backward_compatibility": {
			Advisor:  &SchemaBackwardCompatibilityAdvisor{},
			RuleType: advisor.SchemaRuleSchemaBackwardCompatibility,
			Title:    string(advisor.SchemaRuleSchemaBackwardCompatibility),
		},
		"statement_affected_row_limit": {
			Advisor:  &StatementAffectedRowLimitAdvisor{},
			RuleType: advisor.SchemaRuleStatementAffectedRowLimit,
			Title:    string(advisor.SchemaRuleStatementAffectedRowLimit),
		},
		"system_charset_allowlist": {
			Advisor:  &SystemCharsetAllowlistAdvisor{},
			RuleType: advisor.SchemaRuleCharsetAllowlist,
			Title:    string(advisor.SchemaRuleCharsetAllowlist),
		},
		"system_collation_allowlist": {
			Advisor:  &SystemCollationAllowlistAdvisor{},
			RuleType: advisor.SchemaRuleCollationAllowlist,
			Title:    string(advisor.SchemaRuleCollationAllowlist),
		},
		"statement_require_algorithm_option": {
			Advisor:  &StatementRequireAlgorithmOptionAdvisor{},
			RuleType: advisor.SchemaRuleStatementRequireAlgorithmOption,
			Title:    string(advisor.SchemaRuleStatementRequireAlgorithmOption),
		},
		"statement_require_lock_option": {
			Advisor:  &StatementRequireLockOptionAdvisor{},
			RuleType: advisor.SchemaRuleStatementRequireLockOption,
			Title:    string(advisor.SchemaRuleStatementRequireLockOption),
		},
		"statement_where_maximum_logical_operator_count": {
			Advisor:  &StatementWhereMaximumLogicalOperatorCountAdvisor{},
			RuleType: advisor.SchemaRuleStatementWhereMaximumLogicalOperatorCount,
			Title:    string(advisor.SchemaRuleStatementWhereMaximumLogicalOperatorCount),
		},
		"statement_where_no_leading_wildcard_like": {
			Advisor:  &StatementWhereNoLeadingWildcardLikeAdvisor{},
			RuleType: advisor.SchemaRuleStatementNoLeadingWildcardLike,
			Title:    string(advisor.SchemaRuleStatementNoLeadingWildcardLike),
		},
		"table_disallow_partition": {
			Advisor:  &TableDisallowPartitionAdvisor{},
			RuleType: advisor.SchemaRuleTableDisallowPartition,
			Title:    string(advisor.SchemaRuleTableDisallowPartition),
		},
		"table_disallow_trigger": {
			Advisor:  &TableDisallowTriggerAdvisor{},
			RuleType: advisor.SchemaRuleTableDisallowTrigger,
			Title:    string(advisor.SchemaRuleTableDisallowTrigger),
		},
		"table_disallow_set_charset": {
			Advisor:  &TableDisallowSetCharsetAdvisor{},
			RuleType: advisor.SchemaRuleTableDisallowSetCharset,
			Title:    string(advisor.SchemaRuleTableDisallowSetCharset),
		},
		"table_drop_naming_convention": {
			Advisor:  &TableDropNamingConventionAdvisor{},
			RuleType: advisor.SchemaRuleTableDropNamingConvention,
			Title:    string(advisor.SchemaRuleTableDropNamingConvention),
		},
		"column_require_charset": {
			Advisor:  &ColumnRequireCharsetAdvisor{},
			RuleType: advisor.SchemaRuleColumnRequireCharset,
			Title:    string(advisor.SchemaRuleColumnRequireCharset),
		},
		"column_require_collation": {
			Advisor:  &ColumnRequireCollationAdvisor{},
			RuleType: advisor.SchemaRuleColumnRequireCollation,
			Title:    string(advisor.SchemaRuleColumnRequireCollation),
		},
		"statement_add_column_without_position": {
			Advisor:  &StatementAddColumnWithoutPositionAdvisor{},
			RuleType: advisor.SchemaRuleStatementAddColumnWithoutPosition,
			Title:    string(advisor.SchemaRuleStatementAddColumnWithoutPosition),
		},
		"table_require_charset": {
			Advisor:  &TableRequireCharsetAdvisor{},
			RuleType: advisor.SchemaRuleTableRequireCharset,
			Title:    string(advisor.SchemaRuleTableRequireCharset),
		},
		"table_require_collation": {
			Advisor:  &TableRequireCollationAdvisor{},
			RuleType: advisor.SchemaRuleTableRequireCollation,
			Title:    string(advisor.SchemaRuleTableRequireCollation),
		},
		"table_disallow_ddl": {
			Advisor:  &TableDisallowDDLAdvisor{},
			RuleType: advisor.SchemaRuleTableDisallowDDL,
			Title:    string(advisor.SchemaRuleTableDisallowDDL),
		},
		"table_disallow_dml": {
			Advisor:  &TableDisallowDMLAdvisor{},
			RuleType: advisor.SchemaRuleTableDisallowDML,
			Title:    string(advisor.SchemaRuleTableDisallowDML),
		},
		"table_text_fields_total_length": {
			Advisor:  &TableTextFieldsTotalLengthAdvisor{},
			RuleType: advisor.SchemaRuleTableTextFieldsTotalLength,
			Title:    string(advisor.SchemaRuleTableTextFieldsTotalLength),
		},
		"table_limit_size": {
			Advisor:  &TableLimitSizeAdvisor{},
			RuleType: advisor.SchemaRuleTableLimitSize,
			Title:    string(advisor.SchemaRuleTableLimitSize),
		},

		// TODO: Add more rules here as they are updated to the new signature
	}
}

// TestMySQLRulesFromYAML tests all implemented rules using YAML files
func TestMySQLRulesFromYAML(t *testing.T) {
	ruleMappings := GetRuleMappings()

	for ruleName, mapping := range ruleMappings {
		t.Run(ruleName, func(t *testing.T) {
			runRuleTest(t, ruleName, mapping, false /* record */)
		})
	}
}

// runRuleTest runs a single rule test from its YAML file
func runRuleTest(t *testing.T, ruleName string, mapping RuleMapping, record bool) {
	var tests []TestCase

	// Read the YAML test file
	testFile := filepath.Join("testdata", ruleName+".yaml")
	yamlFile, err := os.Open(testFile)
	if os.IsNotExist(err) {
		t.Skipf("Test file %s does not exist", testFile)
		return
	}
	require.NoError(t, err)
	defer func() {
		_ = yamlFile.Close()
	}()

	byteValue, err := io.ReadAll(yamlFile)
	require.NoError(t, err)
	err = yaml.Unmarshal(byteValue, &tests)
	require.NoError(t, err)

	for i, tc := range tests {
		t.Run(unifiedFormatTestName(i, tc.Statement), func(t *testing.T) {
			var adviceList []*types.Advice
			var err error

			// Create SQL review rule
			rule := &types.SQLReviewRule{
				Type:  string(mapping.RuleType),
				Level: types.SQLReviewRuleLevel_WARNING,
			}

			// Set default payload for rules that require them
			if payload, err := SetDefaultSQLReviewRulePayload(mapping.RuleType); err == nil && payload != nil {
				rule.Payload = payload
			}

			// Create check context with appropriate mock database schema
			var mockDB *types.DatabaseSchemaMetadata
			if mapping.RuleType == advisor.SchemaRuleIndexTotalNumberLimit {
				mockDB = CreateMockDatabaseForIndexTotalNumberLimit()
			} else {
				mockDB = CreateMockDatabaseForTableRequirePK()
			}

			checkContext := advisor.Context{
				DBSchema:   mockDB,
				DBType:     types.Engine_MYSQL,
				ChangeType: types.PlanCheckRunConfig_ChangeDatabaseType(tc.ChangeType),
				Rule:       rule,
				Statements: tc.Statement,
			}

			// Run the advisor check
			if mapping.Advisor != nil {
				adviceList, err = mapping.Advisor.Check(context.Background(), tc.Statement, rule, checkContext)
			} else {
				t.Fatalf("No advisor defined for rule %s", ruleName)
			}
			require.NoError(t, err)

			// Sort adviceList by (line, content) to match Bytebase behavior
			slices.SortFunc(adviceList, func(x, y *types.Advice) int {
				if x.StartPosition == nil || y.StartPosition == nil {
					if x.StartPosition == nil && y.StartPosition == nil {
						return 0
					} else if x.StartPosition == nil {
						return -1
					}
					return 1
				}
				if x.StartPosition.Line != y.StartPosition.Line {
					if x.StartPosition.Line < y.StartPosition.Line {
						return -1
					}
					return 1
				}
				if x.Content < y.Content {
					return -1
				} else if x.Content > y.Content {
					return 1
				}
				return 0
			})

			if record {
				// Record mode: update the test case with actual results
				tests[i].Want = adviceList
			} else {
				// Test mode: compare expected vs actual
				require.Equalf(t, len(tc.Want), len(adviceList), "rule: %s, statement: %s", ruleName, tc.Statement)

				for j, advice := range adviceList {
					if j >= len(tc.Want) {
						t.Errorf("Extra advice[%d]: %s", j, advice.Content)
						continue
					}

					want := tc.Want[j]
					require.Equalf(t, want.Status, advice.Status, "Advice[%d].Status mismatch", j)
					require.Equalf(t, want.Code, advice.Code, "Advice[%d].Code mismatch", j)
					require.Equalf(t, want.Title, advice.Title, "Advice[%d].Title mismatch", j)
					require.Equalf(t, want.Content, advice.Content, "Advice[%d].Content mismatch", j)

					if want.StartPosition != nil {
						require.NotNilf(t, advice.StartPosition, "Advice[%d].StartPosition should not be nil", j)
						require.Equalf(t, want.StartPosition.Line, advice.StartPosition.Line, "Advice[%d].StartPosition.Line mismatch", j)
						require.Equalf(t, want.StartPosition.Column, advice.StartPosition.Column, "Advice[%d].StartPosition.Column mismatch", j)
					}
				}
			}
		})
	}

	if record {
		// Write back the updated test cases to the YAML file
		err := yamlFile.Close()
		require.NoError(t, err)
		byteValue, err := yaml.Marshal(tests)
		require.NoError(t, err)
		err = os.WriteFile(testFile, byteValue, 0o644)
		require.NoError(t, err)
	}
}

// unifiedFormatTestName creates a readable test name from the test case
func unifiedFormatTestName(_ int, statement string) string {
	// Extract first line for the test name
	var firstLine []rune
	for _, r := range statement {
		if r == '\n' {
			break
		}
		firstLine = append(firstLine, r)
	}

	if len(firstLine) > 60 {
		firstLine = firstLine[:57]
		firstLine = append(firstLine, []rune("...")...)
	}

	return string(firstLine)
}

// TestMySQLRulesRecord is a helper function to run all rules in record mode
func TestMySQLRulesRecord(t *testing.T) {
	t.Skip("Uncomment this test and run it to record new test expectations for all rules")

	ruleMappings := GetRuleMappings()

	for ruleName, mapping := range ruleMappings {
		t.Run("record_"+ruleName, func(t *testing.T) {
			runRuleTest(t, ruleName, mapping, true /* record */)
		})
	}
}

// Individual rule test functions for specific testing
func TestMySQLColumnNoNullRule(t *testing.T) {
	mapping := GetRuleMappings()["column_no_null"]
	runRuleTest(t, "column_no_null", mapping, false)
}

func TestMySQLStatementSelectNoSelectAllRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_select_no_select_all"]
	runRuleTest(t, "statement_select_no_select_all", mapping, false)
}

func TestMySQLNamingTableRule(t *testing.T) {
	mapping := GetRuleMappings()["naming_table"]
	runRuleTest(t, "naming_table", mapping, false)
}

func TestMySQLStatementWhereRequireSelectRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_where_require_select"]
	runRuleTest(t, "statement_where_require_select", mapping, false)
}

func TestMySQLStatementWhereRequireUpdateDeleteRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_where_require_update_delete"]
	runRuleTest(t, "statement_where_require_update_delete", mapping, false)
}

func TestMySQLTableRequirePKRule(t *testing.T) {
	mapping := GetRuleMappings()["table_require_pk"]
	runRuleTest(t, "table_require_pk", mapping, false)
}

func TestMySQLColumnCommentRule(t *testing.T) {
	mapping := GetRuleMappings()["column_comment"]
	runRuleTest(t, "column_comment", mapping, false)
}

func TestMySQLColumnRequireDefaultRule(t *testing.T) {
	mapping := GetRuleMappings()["column_require_default"]
	runRuleTest(t, "column_require_default", mapping, false)
}

func TestMySQLColumnRequiredRule(t *testing.T) {
	mapping := GetRuleMappings()["column_required"]
	runRuleTest(t, "column_required", mapping, false)
}

func TestMySQLColumnSetDefaultForNotNullRule(t *testing.T) {
	mapping := GetRuleMappings()["column_set_default_for_not_null"]
	runRuleTest(t, "column_set_default_for_not_null", mapping, false)
}

func TestMySQLColumnDisallowChangeTypeRule(t *testing.T) {
	// Create advisor with mock catalog for this specific test
	ruleAdvisor := &ColumnDisallowChangeTypeAdvisor{}

	// Read the YAML test file
	var tests []TestCase
	testFile := filepath.Join("testdata", "column_disallow_change_type.yaml")
	yamlFile, err := os.Open(testFile)
	require.NoError(t, err)
	defer func() {
		_ = yamlFile.Close()
	}()

	byteValue, err := io.ReadAll(yamlFile)
	require.NoError(t, err)
	err = yaml.Unmarshal(byteValue, &tests)
	require.NoError(t, err)

	for i, tc := range tests {
		t.Run(unifiedFormatTestName(i, tc.Statement), func(t *testing.T) {
			// Create SQL review rule
			rule := &types.SQLReviewRule{
				Type:  string(advisor.SchemaRuleColumnDisallowChangeType),
				Level: types.SQLReviewRuleLevel_WARNING,
			}

			// Create check context with mock database schema
			checkContext := advisor.Context{
				DBSchema:   CreateMockDatabaseForTableRequirePK(),
				DBType:     types.Engine_MYSQL,
				ChangeType: types.PlanCheckRunConfig_ChangeDatabaseType(tc.ChangeType),
				Rule:       rule,
				Statements: tc.Statement,
			}

			// Use the new Check method signature
			adviceList, err := ruleAdvisor.Check(context.Background(), tc.Statement, rule, checkContext)
			require.NoError(t, err)

			// Sort adviceList by (line, content) to match Bytebase behavior
			slices.SortFunc(adviceList, func(x, y *types.Advice) int {
				if x.StartPosition == nil || y.StartPosition == nil {
					if x.StartPosition == nil && y.StartPosition == nil {
						return 0
					} else if x.StartPosition == nil {
						return -1
					}
					return 1
				}
				if x.StartPosition.Line != y.StartPosition.Line {
					if x.StartPosition.Line < y.StartPosition.Line {
						return -1
					}
					return 1
				}
				if x.Content < y.Content {
					return -1
				} else if x.Content > y.Content {
					return 1
				}
				return 0
			})

			// Compare expected vs actual
			require.Equalf(t, len(tc.Want), len(adviceList), "rule: column_disallow_change_type, statement: %s", tc.Statement)

			for j, advice := range adviceList {
				if j >= len(tc.Want) {
					t.Errorf("Extra advice[%d]: %s", j, advice.Content)
					continue
				}

				want := tc.Want[j]
				require.Equalf(t, want.Status, advice.Status, "Advice[%d].Status mismatch", j)
				require.Equalf(t, want.Code, advice.Code, "Advice[%d].Code mismatch", j)
				require.Equalf(t, want.Title, advice.Title, "Advice[%d].Title mismatch", j)
				require.Equalf(t, want.Content, advice.Content, "Advice[%d].Content mismatch", j)

				if want.StartPosition != nil {
					require.NotNilf(t, advice.StartPosition, "Advice[%d].StartPosition should not be nil", j)
					require.Equalf(
						t,
						want.StartPosition.Line,
						advice.StartPosition.Line,
						"Advice[%d].StartPosition.Line mismatch",
						j,
					)
					require.Equalf(
						t,
						want.StartPosition.Column,
						advice.StartPosition.Column,
						"Advice[%d].StartPosition.Column mismatch",
						j,
					)
				}
			}
		})
	}
}

func TestMySQLIndexNoDuplicateColumnRule(t *testing.T) {
	mapping := GetRuleMappings()["index_no_duplicate_column"]
	runRuleTest(t, "index_no_duplicate_column", mapping, false)
}

func TestMySQLNamingColumnRule(t *testing.T) {
	mapping := GetRuleMappings()["naming_column"]
	runRuleTest(t, "naming_column", mapping, false)
}

func TestMySQLTableCommentRule(t *testing.T) {
	mapping := GetRuleMappings()["table_comment"]
	runRuleTest(t, "table_comment", mapping, false)
}

func TestMySQLColumnAutoIncrementInitialValueRule(t *testing.T) {
	mapping := GetRuleMappings()["column_auto_increment_initial_value"]
	runRuleTest(t, "column_auto_increment_initial_value", mapping, false)
}

func TestMySQLColumnAutoIncrementMustIntegerRule(t *testing.T) {
	mapping := GetRuleMappings()["column_auto_increment_must_integer"]
	runRuleTest(t, "column_auto_increment_must_integer", mapping, false)
}

func TestMySQLColumnAutoIncrementMustUnsignedRule(t *testing.T) {
	mapping := GetRuleMappings()["column_auto_increment_must_unsigned"]
	runRuleTest(t, "column_auto_increment_must_unsigned", mapping, false)
}

func TestMySQLEngineMySQLUseInnoDBRule(t *testing.T) {
	mapping := GetRuleMappings()["engine_mysql_use_innodb"]
	runRuleTest(t, "engine_mysql_use_innodb", mapping, false)
}

func TestMySQLIndexKeyNumberLimitRule(t *testing.T) {
	mapping := GetRuleMappings()["index_key_number_limit"]
	runRuleTest(t, "index_key_number_limit", mapping, false)
}

// Helper functions for record mode for individual rules
func TestMySQLColumnNoNullRuleRecord(t *testing.T) {
	t.Skip("Uncomment to record column_no_null rule expectations")
	mapping := GetRuleMappings()["column_no_null"]
	runRuleTest(t, "column_no_null", mapping, true)
}

func TestMySQLStatementSelectNoSelectAllRuleRecord(t *testing.T) {
	t.Skip("Uncomment to record statement_select_no_select_all rule expectations")
	mapping := GetRuleMappings()["statement_select_no_select_all"]
	runRuleTest(t, "statement_select_no_select_all", mapping, true)
}

func TestMySQLNamingTableRuleRecord(t *testing.T) {
	t.Skip("Uncomment to record naming_table rule expectations")
	mapping := GetRuleMappings()["naming_table"]
	runRuleTest(t, "naming_table", mapping, true)
}

func TestMySQLColumnDisallowChangingOrderRule(t *testing.T) {
	mapping := GetRuleMappings()["column_disallow_changing_order"]
	runRuleTest(t, "column_disallow_changing_order", mapping, false)
}

func TestMySQLColumnDisallowDropInIndexRule(t *testing.T) {
	mapping := GetRuleMappings()["column_disallow_drop_in_index"]
	runRuleTest(t, "column_disallow_drop_in_index", mapping, false)
}

func TestMySQLColumnDisallowSetCharsetRule(t *testing.T) {
	mapping := GetRuleMappings()["column_disallow_set_charset"]
	runRuleTest(t, "column_disallow_set_charset", mapping, false)
}

func TestMySQLColumnMaximumCharacterLengthRule(t *testing.T) {
	mapping := GetRuleMappings()["column_maximum_character_length"]
	runRuleTest(t, "column_maximum_character_length", mapping, false)
}

func TestMySQLColumnMaximumVarcharLengthRule(t *testing.T) {
	mapping := GetRuleMappings()["column_maximum_varchar_length"]
	runRuleTest(t, "column_maximum_varchar_length", mapping, false)
}

func TestMySQLColumnTypeDisallowListRule(t *testing.T) {
	mapping := GetRuleMappings()["column_type_disallow_list"]
	runRuleTest(t, "column_type_disallow_list", mapping, false)
}

func TestMySQLDatabaseDropEmptyDatabaseRule(t *testing.T) {
	mapping := GetRuleMappings()["database_drop_empty_database"]
	runRuleTest(t, "database_drop_empty_database", mapping, false)
}

func TestMySQLIndexPkTypeLimitRule(t *testing.T) {
	mapping := GetRuleMappings()["index_pk_type_limit"]
	runRuleTest(t, "index_pk_type_limit", mapping, false)
}

func TestMySQLIndexPrimaryKeyTypeAllowlistRule(t *testing.T) {
	mapping := GetRuleMappings()["index_primary_key_type_allowlist"]
	runRuleTest(t, "index_primary_key_type_allowlist", mapping, false)
}

func TestMySQLIndexTotalNumberLimitRule(t *testing.T) {
	// Create advisor with mock catalog for this specific test
	ruleAdvisor := &IndexTotalNumberLimitAdvisor{}

	// Read the YAML test file
	var tests []TestCase
	testFile := filepath.Join("testdata", "index_total_number_limit.yaml")
	yamlFile, err := os.Open(testFile)
	require.NoError(t, err)
	defer func() {
		_ = yamlFile.Close()
	}()

	byteValue, err := io.ReadAll(yamlFile)
	require.NoError(t, err)
	err = yaml.Unmarshal(byteValue, &tests)
	require.NoError(t, err)

	for i, tc := range tests {
		t.Run(unifiedFormatTestName(i, tc.Statement), func(t *testing.T) {
			// Create SQL review rule
			rule := &types.SQLReviewRule{
				Type:  string(advisor.SchemaRuleIndexTotalNumberLimit),
				Level: types.SQLReviewRuleLevel_WARNING,
			}

			// Set payload for max index count
			if payload, err := SetDefaultSQLReviewRulePayload(advisor.SchemaRuleIndexTotalNumberLimit); err == nil &&
				payload != nil {
				rule.Payload = payload
			}

			// Create check context with mock database schema specifically for index tests
			checkContext := advisor.Context{
				DBSchema:   CreateMockDatabaseForIndexTotalNumberLimit(),
				DBType:     types.Engine_MYSQL,
				ChangeType: types.PlanCheckRunConfig_ChangeDatabaseType(tc.ChangeType),
				Rule:       rule,
				Statements: tc.Statement,
			}

			// Use the Check method
			adviceList, err := ruleAdvisor.Check(context.Background(), tc.Statement, rule, checkContext)
			require.NoError(t, err)

			// Sort adviceList by (line, content) to match Bytebase behavior
			slices.SortFunc(adviceList, func(x, y *types.Advice) int {
				if x.StartPosition == nil || y.StartPosition == nil {
					if x.StartPosition == nil && y.StartPosition == nil {
						return 0
					} else if x.StartPosition == nil {
						return -1
					}
					return 1
				}
				if x.StartPosition.Line != y.StartPosition.Line {
					if x.StartPosition.Line < y.StartPosition.Line {
						return -1
					}
					return 1
				}
				if x.Content < y.Content {
					return -1
				} else if x.Content > y.Content {
					return 1
				}
				return 0
			})

			// Test mode: compare expected vs actual
			require.Equalf(t, len(tc.Want), len(adviceList), "rule: index_total_number_limit, statement: %s", tc.Statement)

			for j, advice := range adviceList {
				if j >= len(tc.Want) {
					t.Errorf("Extra advice[%d]: %s", j, advice.Content)
					continue
				}

				expected := tc.Want[j]
				require.Equalf(
					t,
					int32(expected.Status),
					int32(advice.Status),
					"advice[%d].Status, expected: %d, actual: %d",
					j,
					expected.Status,
					advice.Status,
				)
				require.Equalf(
					t,
					expected.Code,
					advice.Code,
					"advice[%d].Code, expected: %d, actual: %d",
					j,
					expected.Code,
					advice.Code,
				)
				require.Equalf(
					t,
					expected.Title,
					advice.Title,
					"advice[%d].Title, expected: %s, actual: %s",
					j,
					expected.Title,
					advice.Title,
				)
				require.Equalf(
					t,
					expected.Content,
					advice.Content,
					"advice[%d].Content, expected: %s, actual: %s",
					j,
					expected.Content,
					advice.Content,
				)
				if expected.StartPosition != nil && advice.StartPosition != nil {
					require.Equalf(
						t,
						expected.StartPosition.Line,
						advice.StartPosition.Line,
						"advice[%d].StartPosition.Line, expected: %d, actual: %d",
						j,
						expected.StartPosition.Line,
						advice.StartPosition.Line,
					)
					require.Equalf(
						t,
						expected.StartPosition.Column,
						advice.StartPosition.Column,
						"advice[%d].StartPosition.Column, expected: %d, actual: %d",
						j,
						expected.StartPosition.Column,
						advice.StartPosition.Column,
					)
				}
			}
		})
	}
}

func TestMySQLIndexTypeAllowListRule(t *testing.T) {
	mapping := GetRuleMappings()["index_type_allow_list"]
	runRuleTest(t, "index_type_allow_list", mapping, false)
}

func TestMySQLIndexTypeNoBlobRule(t *testing.T) {
	mapping := GetRuleMappings()["index_type_no_blob"]
	runRuleTest(t, "index_type_no_blob", mapping, false)
}

func TestMySQLNamingColumnAutoIncrementRule(t *testing.T) {
	mapping := GetRuleMappings()["naming_column_auto_increment"]
	runRuleTest(t, "naming_column_auto_increment", mapping, false)
}

func TestMySQLNamingIdentifierNoKeywordRule(t *testing.T) {
	mapping := GetRuleMappings()["naming_identifier_no_keyword"]
	runRuleTest(t, "naming_identifier_no_keyword", mapping, false)
}

func TestMySQLNamingIndexFKRule(t *testing.T) {
	mapping := GetRuleMappings()["naming_index_fk"]
	runRuleTest(t, "naming_index_fk", mapping, false)
}

func TestMySQLNamingIndexIdxRule(t *testing.T) {
	mapping := GetRuleMappings()["naming_index_idx"]
	runRuleTest(t, "naming_index_idx", mapping, false)
}

func TestMySQLNamingIndexUKRule(t *testing.T) {
	mapping := GetRuleMappings()["naming_index_uk"]
	runRuleTest(t, "naming_index_uk", mapping, false)
}

func TestMySQLStatementWhereNoEqualNullRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_where_no_equal_null"]
	runRuleTest(t, "statement_where_no_equal_null", mapping, false)
}

func TestMySQLStatementDisallowCommitRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_commit"]
	runRuleTest(t, "statement_disallow_commit", mapping, false)
}

func TestMySQLStatementDisallowLimitRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_limit"]
	runRuleTest(t, "statement_disallow_limit", mapping, false)
}

func TestMySQLStatementDisallowOrderByRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_order_by"]
	runRuleTest(t, "statement_disallow_order_by", mapping, false)
}

func TestMySQLStatementInsertMustSpecifyColumnRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_insert_must_specify_column"]
	runRuleTest(t, "statement_insert_must_specify_column", mapping, false)
}

func TestMySQLStatementInsertDisallowOrderByRandRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_insert_disallow_order_by_rand"]
	runRuleTest(t, "statement_insert_disallow_order_by_rand", mapping, false)
}

func TestMySQLStatementMaximumLimitValueRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_maximum_limit_value"]
	runRuleTest(t, "statement_maximum_limit_value", mapping, true)
}

func TestMySQLStatementMaximumJoinTableCountRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_maximum_join_table_count"]
	runRuleTest(t, "statement_maximum_join_table_count", mapping, true)
}

func TestMySQLStatementMaximumStatementsInTransactionRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_maximum_statements_in_transaction"]
	runRuleTest(t, "statement_maximum_statements_in_transaction", mapping, true)
}

func TestMySQLStatementQueryMinimumPlanLevelRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_query_minimum_plan_level"]
	runRuleTest(t, "statement_query_minimum_plan_level", mapping, true)
}

func TestMySQLStatementDisallowUsingFilesortRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_using_filesort"]
	runRuleTest(t, "statement_disallow_using_filesort", mapping, false)
}

func TestMySQLStatementDisallowUsingTemporaryRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_using_temporary"]
	runRuleTest(t, "statement_disallow_using_temporary", mapping, false)
}

func TestMySQLStatementJoinStrictColumnAttrsRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_join_strict_column_attrs"]
	runRuleTest(t, "statement_join_strict_column_attrs", mapping, false)
}

func TestMySQLStatementSelectFullTableScanRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_select_full_table_scan"]
	runRuleTest(t, "statement_select_full_table_scan", mapping, false)
}

func TestMySQLStatementMaxExecutionTimeRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_max_execution_time"]
	runRuleTest(t, "statement_max_execution_time", mapping, false)
}

func TestMySQLStatementWhereDisallowFunctionsAndCalculationsRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_where_disallow_functions_and_calculations"]
	runRuleTest(t, "statement_where_disallow_functions_and_calculations", mapping, false)
}

func TestMySQLStatementMergeAlterTableRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_merge_alter_table"]
	runRuleTest(t, "statement_merge_alter_table", mapping, false)
}

func TestMySQLStatementDisallowMixInDDLRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_mix_in_ddl"]
	runRuleTest(t, "statement_disallow_mix_in_ddl", mapping, false)
}

func TestMySQLStatementWhereNoLeadingWildcardLikeRule(t *testing.T) {
	mapping := GetRuleMappings()["statement_where_no_leading_wildcard_like"]
	runRuleTest(t, "statement_where_no_leading_wildcard_like", mapping, false)
}

func TestMySQLTableDisallowPartitionRule(t *testing.T) {
	mapping := GetRuleMappings()["table_disallow_partition"]
	runRuleTest(t, "table_disallow_partition", mapping, false)
}

func TestMySQLTableDisallowTriggerRule(t *testing.T) {
	mapping := GetRuleMappings()["table_disallow_trigger"]
	runRuleTest(t, "table_disallow_trigger", mapping, false)
}

func TestMySQLTableDisallowSetCharsetRule(t *testing.T) {
	mapping := GetRuleMappings()["table_disallow_set_charset"]
	runRuleTest(t, "table_disallow_set_charset", mapping, false)
}

func TestMySQLTableDropNamingConventionRule(t *testing.T) {
	mapping := GetRuleMappings()["table_drop_naming_convention"]
	runRuleTest(t, "table_drop_naming_convention", mapping, false)
}

func TestMySQLSystemEventDisallowCreateRule(t *testing.T) {
	mapping := GetRuleMappings()["system_event_disallow_create"]
	runRuleTest(t, "system_event_disallow_create", mapping, false)
}

func TestMySQLSystemFunctionDisallowCreateRule(t *testing.T) {
	mapping := GetRuleMappings()["system_function_disallow_create"]
	runRuleTest(t, "system_function_disallow_create", mapping, false)
}

func TestMySQLSystemFunctionDisallowedListRule(t *testing.T) {
	mapping := GetRuleMappings()["system_function_disallowed_list"]
	runRuleTest(t, "system_function_disallowed_list", mapping, true)
}

func TestMySQLSystemProcedureDisallowCreateRule(t *testing.T) {
	mapping := GetRuleMappings()["system_procedure_disallow_create"]
	runRuleTest(t, "system_procedure_disallow_create", mapping, false)
}

func TestMySQLSystemViewDisallowCreateRule(t *testing.T) {
	mapping := GetRuleMappings()["system_view_disallow_create"]
	runRuleTest(t, "system_view_disallow_create", mapping, false)
}

func TestStatementDisallowMixInDDL(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_mix_in_ddl"]
	runRuleTest(t, "statement_disallow_mix_in_ddl", mapping, false)
}

func TestStatementDisallowMixInDML(t *testing.T) {
	mapping := GetRuleMappings()["statement_disallow_mix_in_dml"]
	runRuleTest(t, "statement_disallow_mix_in_dml", mapping, false)
}

func TestStatementInsertRowLimit(t *testing.T) {
	mapping := GetRuleMappings()["statement_insert_row_limit"]
	runRuleTest(t, "statement_insert_row_limit", mapping, true)
}

func TestStatementDmlDryRun(t *testing.T) {
	mapping := GetRuleMappings()["statement_dml_dry_run"]
	runRuleTest(t, "statement_dml_dry_run", mapping, true)
}

func TestTableNoDuplicateIndex(t *testing.T) {
	mapping := GetRuleMappings()["table_no_duplicate_index"]
	runRuleTest(t, "table_no_duplicate_index", mapping, true)
}

func TestSchemaBackwardCompatibility(t *testing.T) {
	mapping := GetRuleMappings()["schema_backward_compatibility"]
	runRuleTest(t, "schema_backward_compatibility", mapping, true)
}

func TestStatementAffectedRowLimit(t *testing.T) {
	mapping := GetRuleMappings()["statement_affected_row_limit"]
	runRuleTest(t, "statement_affected_row_limit", mapping, true)
}

func TestSystemCharsetAllowlist(t *testing.T) {
	mapping := GetRuleMappings()["system_charset_allowlist"]
	runRuleTest(t, "system_charset_allowlist", mapping, true)
}

func TestSystemCollationAllowlist(t *testing.T) {
	mapping := GetRuleMappings()["system_collation_allowlist"]
	runRuleTest(t, "system_collation_allowlist", mapping, true)
}

func TestStatementRequireAlgorithmOption(t *testing.T) {
	mapping := GetRuleMappings()["statement_require_algorithm_option"]
	runRuleTest(t, "statement_require_algorithm_option", mapping, false)
}

func TestStatementRequireLockOption(t *testing.T) {
	mapping := GetRuleMappings()["statement_require_lock_option"]
	runRuleTest(t, "statement_require_lock_option", mapping, false)
}

func TestStatementWhereMaximumLogicalOperatorCount(t *testing.T) {
	mapping := GetRuleMappings()["statement_where_maximum_logical_operator_count"]
	runRuleTest(t, "statement_where_maximum_logical_operator_count", mapping, true)
}

// TestAvailableRules lists all available rules and their test files
func TestAvailableRules(t *testing.T) {
	ruleMappings := GetRuleMappings()

	t.Logf("Available rules and their test files:")
	for ruleName, mapping := range ruleMappings {
		testFile := filepath.Join("testdata", ruleName+".yaml")
		if _, err := os.Stat(testFile); err == nil {
			t.Logf("✓ %s -> %s (rule type: %s)", ruleName, testFile, mapping.RuleType)
		} else {
			t.Logf("✗ %s -> %s (missing, rule type: %s)", ruleName, testFile, mapping.RuleType)
		}
	}

	// List all YAML files in testdata
	testDataDir := "testdata"
	files, err := os.ReadDir(testDataDir)
	require.NoError(t, err)

	t.Logf("\nAll test files in testdata:")
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".yaml" {
			ruleName := strings.TrimSuffix(file.Name(), ".yaml")
			ruleType := yamlFileNameToRuleType(ruleName)
			if mapping, exists := ruleMappings[ruleName]; exists {
				t.Logf("✓ %s (implemented, rule type: %s)", file.Name(), mapping.RuleType)
			} else {
				t.Logf("✗ %s (not implemented, expected rule type: %s)", file.Name(), ruleType)
			}
		}
	}
}
