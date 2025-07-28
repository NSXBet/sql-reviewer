package mysql

import (
	"context"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// init registers MySQL rules with the advisor system
func init() {
	// Register basic MySQL rules for testing
	registerBasicMySQLRules()
}

// MySQLAdvisorWrapper wraps a MySQL advisor to implement the CLI advisor interface
type MySQLAdvisorWrapper struct {
	checkFn func(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error)
}

// Check implements the advisor interface
func (w *MySQLAdvisorWrapper) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	// MySQL advisors expect the rule and statements as separate parameters
	return w.checkFn(ctx, checkCtx.Statements, checkCtx.Rule, checkCtx)
}

// registerBasicMySQLRules registers all MySQL rule implementations
func registerBasicMySQLRules() {
	// Column rules
	registerMySQLRule(&ColumnAutoIncrementInitialValueAdvisor{}, advisor.SchemaRuleColumnAutoIncrementInitialValue)
	registerMySQLRule(&ColumnAutoIncrementMustIntegerAdvisor{}, advisor.SchemaRuleColumnAutoIncrementMustInteger)
	registerMySQLRule(&ColumnAutoIncrementMustUnsignedAdvisor{}, advisor.SchemaRuleColumnAutoIncrementMustUnsigned)
	registerMySQLRule(&ColumnCommentAdvisor{}, advisor.SchemaRuleColumnCommentConvention)
	registerMySQLRule(&ColumnCurrentTimeCountLimitAdvisor{}, advisor.SchemaRuleCurrentTimeColumnCountLimit)
	registerMySQLRule(&ColumnDisallowChangeAdvisor{}, advisor.SchemaRuleColumnDisallowChange)
	registerMySQLRule(&ColumnDisallowChangeTypeAdvisor{}, advisor.SchemaRuleColumnDisallowChangeType)
	registerMySQLRule(&ColumnDisallowChangingOrderAdvisor{}, advisor.SchemaRuleColumnDisallowChangingOrder)
	registerMySQLRule(&ColumnDisallowDropAdvisor{}, advisor.SchemaRuleColumnDisallowDrop)
	registerMySQLRule(&ColumnDisallowDropInIndexAdvisor{}, advisor.SchemaRuleColumnDisallowDropInIndex)
	registerMySQLRule(&ColumnDisallowSetCharsetAdvisor{}, advisor.SchemaRuleColumnDisallowSetCharset)
	registerMySQLRule(&ColumnMaximumCharacterLengthAdvisor{}, advisor.SchemaRuleColumnMaximumCharacterLength)
	registerMySQLRule(&ColumnMaximumVarcharLengthAdvisor{}, advisor.SchemaRuleColumnMaximumVarcharLength)
	registerMySQLRule(&ColumnNoNullAdvisor{}, advisor.SchemaRuleColumnNotNull)
	registerMySQLRule(&ColumnRequireCharsetAdvisor{}, advisor.SchemaRuleColumnRequireCharset)
	registerMySQLRule(&ColumnRequireCollationAdvisor{}, advisor.SchemaRuleColumnRequireCollation)
	registerMySQLRule(&ColumnRequireDefaultAdvisor{}, advisor.SchemaRuleColumnRequireDefault)
	registerMySQLRule(&ColumnRequiredAdvisor{}, advisor.SchemaRuleRequiredColumn)
	registerMySQLRule(&ColumnSetDefaultForNotNullAdvisor{}, advisor.SchemaRuleColumnSetDefaultForNotNull)
	registerMySQLRule(&ColumnTypeDisallowListAdvisor{}, advisor.SchemaRuleColumnTypeDisallowList)

	// Database rules
	registerMySQLRule(&DatabaseDropEmptyDatabaseAdvisor{}, advisor.SchemaRuleDropEmptyDatabase)

	// Engine rules
	registerMySQLRule(&EngineMySQLUseInnoDBAdvisor{}, advisor.SchemaRuleMySQLEngine)

	// Index rules
	registerMySQLRule(&IndexKeyNumberLimitAdvisor{}, advisor.SchemaRuleIndexKeyNumberLimit)
	registerMySQLRule(&IndexNoDuplicateColumnAdvisor{}, advisor.SchemaRuleIndexNoDuplicateColumn)
	registerMySQLRule(&IndexPkTypeLimitAdvisor{}, advisor.SchemaRuleIndexPKTypeLimit)
	registerMySQLRule(&IndexPrimaryKeyTypeAllowlistAdvisor{}, advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist)
	registerMySQLRule(&IndexTotalNumberLimitAdvisor{}, advisor.SchemaRuleIndexTotalNumberLimit)
	registerMySQLRule(&IndexTypeAllowListAdvisor{}, advisor.SchemaRuleIndexTypeAllowList)
	registerMySQLRule(&IndexTypeNoBlobAdvisor{}, advisor.SchemaRuleIndexTypeNoBlob)

	// Naming rules (use the advisor types with proper Check method signature)
	registerMySQLRule(&NamingColumnAdvisor{}, advisor.SchemaRuleColumnNaming)
	registerMySQLRule(&NamingAutoIncrementColumnAdvisor{}, advisor.SchemaRuleAutoIncrementColumnNaming)
	registerMySQLRule(&NamingIdentifierNoKeywordAdvisor{}, advisor.SchemaRuleIdentifierNoKeyword)
	registerMySQLRule(&NamingIndexFKAdvisor{}, advisor.SchemaRuleFKNaming)
	registerMySQLRule(&NamingIndexIdxAdvisor{}, advisor.SchemaRuleIDXNaming)
	registerMySQLRule(&NamingIndexUKAdvisor{}, advisor.SchemaRuleUKNaming)
	registerMySQLRule(&NamingTableConventionAdvisor{}, advisor.SchemaRuleTableNaming)

	// Schema rules
	registerMySQLRule(&SchemaBackwardCompatibilityAdvisor{}, advisor.SchemaRuleSchemaBackwardCompatibility)

	// Statement rules
	registerMySQLRule(&StatementAddColumnWithoutPositionAdvisor{}, advisor.SchemaRuleStatementAddColumnWithoutPosition)
	registerMySQLRule(&StatementAffectedRowLimitAdvisor{}, advisor.SchemaRuleStatementAffectedRowLimit)
	registerMySQLRule(&StatementDisallowCommitAdvisor{}, advisor.SchemaRuleStatementDisallowCommit)
	registerMySQLRule(&StatementDisallowLimitAdvisor{}, advisor.SchemaRuleStatementDisallowLimit)
	registerMySQLRule(&StatementDisallowMixInDDLAdvisor{}, advisor.SchemaRuleStatementDisallowMixInDDL)
	registerMySQLRule(&StatementDisallowMixInDMLAdvisor{}, advisor.SchemaRuleStatementDisallowMixInDML)
	registerMySQLRule(&StatementDisallowOrderByAdvisor{}, advisor.SchemaRuleStatementDisallowOrderBy)
	registerMySQLRule(&StatementDisallowUsingFilesortAdvisor{}, advisor.SchemaRuleStatementDisallowUsingFilesort)
	registerMySQLRule(&StatementDisallowUsingTemporaryAdvisor{}, advisor.SchemaRuleStatementDisallowUsingTemporary)
	registerMySQLRule(&StatementDmlDryRunAdvisor{}, advisor.SchemaRuleStatementDMLDryRun)
	registerMySQLRule(&StatementInsertDisallowOrderByRandAdvisor{}, advisor.SchemaRuleStatementInsertDisallowOrderByRand)
	registerMySQLRule(&StatementInsertMustSpecifyColumnAdvisor{}, advisor.SchemaRuleStatementInsertMustSpecifyColumn)
	registerMySQLRule(&StatementInsertRowLimitAdvisor{}, advisor.SchemaRuleStatementInsertRowLimit)
	registerMySQLRule(&StatementJoinStrictColumnAttrsAdvisor{}, advisor.SchemaRuleStatementJoinStrictColumnAttrs)
	registerMySQLRule(&StatementMaxExecutionTimeAdvisor{}, advisor.SchemaRuleStatementMaxExecutionTime)
	registerMySQLRule(&StatementMaximumJoinTableCountAdvisor{}, advisor.SchemaRuleStatementMaximumJoinTableCount)
	registerMySQLRule(&StatementMaximumLimitValueAdvisor{}, advisor.SchemaRuleStatementMaximumLimitValue)
	registerMySQLRule(&StatementMaximumStatementsInTransactionAdvisor{}, advisor.SchemaRuleStatementMaximumStatementsInTransaction)
	registerMySQLRule(&StatementMergeAlterTableAdvisor{}, advisor.SchemaRuleStatementMergeAlterTable)
	registerMySQLRule(&StatementQueryMinumumPlanLevelAdvisor{}, advisor.SchemaRuleStatementQueryMinumumPlanLevel)
	registerMySQLRule(&StatementRequireAlgorithmOptionAdvisor{}, advisor.SchemaRuleStatementRequireAlgorithmOption)
	registerMySQLRule(&StatementRequireLockOptionAdvisor{}, advisor.SchemaRuleStatementRequireLockOption)
	registerMySQLRule(&StatementSelectFullTableScanAdvisor{}, advisor.SchemaRuleStatementSelectFullTableScan)
	registerMySQLRule(&StatementSelectNoSelectAllAdvisor{}, advisor.SchemaRuleStatementNoSelectAll)
	registerMySQLRule(&StatementWhereDisallowFunctionsAndCalculationsAdvisor{}, advisor.SchemaRuleStatementWhereDisallowFunctionsAndCaculations)
	registerMySQLRule(&StatementWhereMaximumLogicalOperatorCountAdvisor{}, advisor.SchemaRuleStatementWhereMaximumLogicalOperatorCount)
	registerMySQLRule(&StatementWhereNoEqualNullAdvisor{}, advisor.SchemaRuleStatementWhereNoEqualNull)
	registerMySQLRule(&StatementWhereNoLeadingWildcardLikeAdvisor{}, advisor.SchemaRuleStatementNoLeadingWildcardLike)
	registerMySQLRule(&StatementWhereRequireSelectAdvisor{}, advisor.SchemaRuleStatementRequireWhereForSelect)
	registerMySQLRule(&StatementWhereRequireUpdateDeleteAdvisor{}, advisor.SchemaRuleStatementRequireWhereForUpdateDelete)

	// System rules
	registerMySQLRule(&SystemCharsetAllowlistAdvisor{}, advisor.SchemaRuleCharsetAllowlist)
	registerMySQLRule(&SystemCollationAllowlistAdvisor{}, advisor.SchemaRuleCollationAllowlist)
	registerMySQLRule(&SystemEventDisallowCreateAdvisor{}, advisor.SchemaRuleEventDisallowCreate)
	registerMySQLRule(&SystemFunctionDisallowCreateAdvisor{}, advisor.SchemaRuleFunctionDisallowCreate)
	registerMySQLRule(&SystemFunctionDisallowedListAdvisor{}, advisor.SchemaRuleFunctionDisallowList)
	registerMySQLRule(&SystemProcedureDisallowCreateAdvisor{}, advisor.SchemaRuleProcedureDisallowCreate)
	registerMySQLRule(&SystemViewDisallowCreateAdvisor{}, advisor.SchemaRuleViewDisallowCreate)

	// Table rules
	registerMySQLRule(&TableCommentAdvisor{}, advisor.SchemaRuleTableCommentConvention)
	registerMySQLRule(&TableDisallowDDLAdvisor{}, advisor.SchemaRuleTableDisallowDDL)
	registerMySQLRule(&TableDisallowDMLAdvisor{}, advisor.SchemaRuleTableDisallowDML)
	registerMySQLRule(&TableDisallowPartitionAdvisor{}, advisor.SchemaRuleTableDisallowPartition)
	registerMySQLRule(&TableDisallowSetCharsetAdvisor{}, advisor.SchemaRuleTableDisallowSetCharset)
	registerMySQLRule(&TableDisallowTriggerAdvisor{}, advisor.SchemaRuleTableDisallowTrigger)
	registerMySQLRule(&TableDropNamingConventionAdvisor{}, advisor.SchemaRuleTableDropNamingConvention)
	registerMySQLRule(&TableLimitSizeAdvisor{}, advisor.SchemaRuleTableLimitSize)
	registerMySQLRule(&TableNoDuplicateIndexAdvisor{}, advisor.SchemaRuleTableNoDuplicateIndex)
	registerMySQLRule(&TableNoForeignKeyAdvisor{}, advisor.SchemaRuleTableNoFK)
	registerMySQLRule(&TableRequireCharsetAdvisor{}, advisor.SchemaRuleTableRequireCharset)
	registerMySQLRule(&TableRequireCollationAdvisor{}, advisor.SchemaRuleTableRequireCollation)
	registerMySQLRule(&TableRequirePKAdvisor{}, advisor.SchemaRuleTableRequirePK)
	registerMySQLRule(&TableTextFieldsTotalLengthAdvisor{}, advisor.SchemaRuleTableTextFieldsTotalLength)
}

// registerMySQLRule is a helper function to register a MySQL rule with the advisor system
func registerMySQLRule(ruleAdvisor interface{}, ruleType advisor.SQLReviewRuleType) {
	// Create a wrapper function that can handle the MySQL advisor interface
	var checkFn func(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error)

	// Use type assertion to get the correct Check method signature
	switch v := ruleAdvisor.(type) {
	case interface {
		Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error)
	}:
		checkFn = v.Check
	case interface {
		Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context, parameter string) ([]*types.Advice, error)
	}:
		// All parameterized rules have been updated to use payload instead of parameter
		// Skip registration as they now use the unified interface above
		return
	default:
		// Skip unknown interface types
		return
	}

	wrapper := &MySQLAdvisorWrapper{
		checkFn: checkFn,
	}
	advisor.Register(types.Engine_MYSQL, advisor.Type(string(ruleType)), wrapper)
}
