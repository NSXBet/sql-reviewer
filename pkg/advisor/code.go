package advisor

// Code is the error code for advisor.
type Code int

// Application error codes for advisor.
const (
	Ok Code = 0

	// 1 ~ 99 general advisor error.
	Internal             Code = 1
	NotFound             Code = 2
	Unsupported          Code = 3
	StatementSyntaxError Code = 4

	// 101 ~ 199 connection error.
	ConnectionDatabaseNotExists Code = 101

	// 201 ~ 299 table error.
	TableExists                       Code = 201
	TableNotExists                    Code = 202
	TableDropNamingConventionMismatch Code = 203

	// 301 ~ 399 column error.
	ColumnNotExists          Code = 301
	ColumnExists             Code = 302
	DropAllColumns           Code = 303
	ColumnIsReferencedByView Code = 304
	InvalidColumnDefault     Code = 305

	// 401 ~ 499 index error.
	PrimaryKeyExists        Code = 401
	IndexExists             Code = 402
	IndexEmptyKeys          Code = 403
	PrimaryKeyNotExists     Code = 404
	IndexNotExists          Code = 405
	IncorrectIndexName      Code = 406
	SpatialIndexKeyNullable Code = 407

	// 501 ~ 599 database error.
	DatabaseNotExists      Code = 501
	NotCurrentDatabase     Code = 502
	DatabaseIsDeleted      Code = 503
	ReferenceOtherDatabase Code = 504

	// 601 ~ 699 view error.
	TableIsReferencedByView Code = 601

	// 10001 ~ 19999 MySQL error.

	// 10001 ~ 10099 MySQL general error.
	MySQLUseInnoDB Code = 10001

	// 10101 ~ 10199 MySQL connection error.

	// 10201 ~ 10299 MySQL table error.
	MySQLTableNamingConventionMismatch          Code = 10201
	MySQLTableNoFK                              Code = 10202
	MySQLTableHasFK                             Code = 10203
	MySQLTableDropNamingConventionMismatch      Code = 10204
	MySQLTableCommentTooLong                    Code = 10205
	MySQLTableCommentRequired                   Code = 10206
	MySQLTableCommentConventionMismatch         Code = 10207
	MySQLTableDisallowPartition                 Code = 10208
	MySQLTableDisallowTrigger                   Code = 10209
	MySQLTableNoDuplicateIndex                  Code = 10210
	MySQLTableTextFieldsTotalLengthExceedsLimit Code = 10211
	MySQLTableDisallowSetCharset                Code = 10212
	MySQLTableRequirePK                         Code = 10213
	MySQLTableNoPK                              Code = 10214
	MySQLTableDisallowDDL                       Code = 10215
	MySQLTableDisallowDML                       Code = 10216
	MySQLTableLimitSize                         Code = 10217
	MySQLTableRequireCharset                    Code = 10218
	MySQLTableCharsetMismatch                   Code = 10219
	MySQLTableRequireCollation                  Code = 10220
	MySQLTableCollationMismatch                 Code = 10221

	// 10301 ~ 10399 MySQL column error.
	MySQLColumnNamingConventionMismatch              Code = 10301
	MySQLAutoIncrementColumnNamingConventionMismatch Code = 10302
	MySQLColumnRequiredMissing                       Code = 10303
	MySQLColumnCannotNull                            Code = 10304
	MySQLColumnDisallowChangingType                  Code = 10305
	MySQLColumnSetDefaultForNotNull                  Code = 10306
	MySQLColumnDisallowChanging                      Code = 10307
	MySQLColumnDisallowChangingOrder                 Code = 10308
	MySQLColumnDisallowDrop                          Code = 10309
	MySQLColumnDisallowDropInIndex                   Code = 10310
	MySQLColumnCommentTooLong                        Code = 10311
	MySQLColumnCommentRequired                       Code = 10312
	MySQLColumnCommentConventionMismatch             Code = 10313
	MySQLAutoIncrementColumnMustInteger              Code = 10314
	MySQLDisallowedColumnType                        Code = 10315
	MySQLColumnDisallowSetCharset                    Code = 10316
	MySQLColumnExceedsMaximumCharacterLength         Code = 10317
	MySQLColumnExceedsMaximumVarcharLength           Code = 10318
	MySQLAutoIncrementColumnInitialValueNotMatch     Code = 10319
	MySQLAutoIncrementColumnMustUnsigned             Code = 10320
	MySQLCurrentTimeColumnCountExceedsLimit          Code = 10321
	MySQLColumnRequireDefault                        Code = 10322
	MySQLColumnNoDefault                             Code = 10323
	MySQLColumnRequireCharset                        Code = 10324
	MySQLColumnCharsetMismatch                       Code = 10325
	MySQLColumnRequireCollation                      Code = 10326
	MySQLColumnCollationMismatch                     Code = 10327

	// 10401 ~ 10499 MySQL index error.
	MySQLIndexNamingConventionMismatch      Code = 10401
	MySQLPrimaryKeyNamingConventionMismatch Code = 10402
	MySQLUniqueKeyNamingConventionMismatch  Code = 10403
	MySQLForeignKeyNamingConventionMismatch Code = 10404
	MySQLIndexNoDuplicateColumn             Code = 10405
	MySQLIndexCountExceedsLimit             Code = 10406
	MySQLIndexPKType                        Code = 10407
	MySQLIndexTypeNoBlob                    Code = 10408
	MySQLIndexTotalNumberExceedsLimit       Code = 10409
	MySQLPrimaryKeyTypeAllowlist            Code = 10410
	MySQLIndexTypeAllowList                 Code = 10411

	// 10501 ~ 10599 MySQL database error.
	MySQLDatabaseNotEmpty Code = 10501

	// 10601 ~ 10699 MySQL statement error.
	MySQLNoSelectAll                               Code = 10601
	MySQLInsertRowLimitExceeds                     Code = 10602
	MySQLInsertMustSpecifyColumn                   Code = 10603
	MySQLInsertDisallowOrderByRand                 Code = 10604
	MySQLDisallowCommit                            Code = 10605
	MySQLDisallowLimit                             Code = 10606
	MySQLDisallowOrderBy                           Code = 10607
	MySQLMergeAlterTable                           Code = 10608
	MySQLStatementAffectedRowLimit                 Code = 10609
	MySQLStatementDMLDryRun                        Code = 10610
	MySQLStatementSelectFullTableScan              Code = 10611
	MySQLStatementWhereNoEqualNull                 Code = 10612
	MySQLStatementWhereDisallowUsingFunction       Code = 10613
	MySQLStatementQueryMinumumPlanLevel            Code = 10614
	MySQLStatementWhereMaximumLogicalOperatorCount Code = 10615
	MySQLStatementMaximumLimitValue                Code = 10616
	MySQLStatementMaximumJoinTableCount            Code = 10617
	MySQLStatementMaximumStatementsInTransaction   Code = 10618
	MySQLStatementJoinStrictColumnAttrs            Code = 10619
	MySQLStatementDisallowMixInDDL                 Code = 10620
	MySQLStatementDisallowMixInDML                 Code = 10621
	MySQLStatementDisallowUsingFilesort            Code = 10622
	MySQLStatementDisallowUsingTemporary           Code = 10623
	MySQLStatementAddColumnWithoutPosition         Code = 10624
	MySQLStatementWhereRequirementForSelect        Code = 10625
	MySQLStatementWhereRequirementForUpdateDelete  Code = 10626
	MySQLStatementNoLeadingWildcardLike            Code = 10627
	MySQLStatementMaxExecutionTime                 Code = 10628
	MySQLStatementRequireAlgorithmOption           Code = 10629
	MySQLStatementRequireLockOption                Code = 10630
	MySQLOnlineMigration                           Code = 10631

	// 10701 ~ 10799 MySQL system error.
	MySQLCharsetAllowlistViolation     Code = 10701
	MySQLCollationAllowlistViolation   Code = 10702
	MySQLMigrationHasRiskySQLStatement Code = 10703
	MySQLDisallowOfflineDDL            Code = 10704
	MySQLProcedureDisallowCreate       Code = 10705
	MySQLEventDisallowCreate           Code = 10706
	MySQLViewDisallowCreate            Code = 10707
	MySQLFunctionDisallowCreate        Code = 10708
	MySQLFunctionDisallowedList        Code = 10709
	MySQLIdentifierNamingNoKeyword     Code = 10710

	// 20001 ~ 29999 PostgreSQL error.

	// 20001 ~ 20099 PostgreSQL general error.

	// 20101 ~ 20199 PostgreSQL connection error.

	// 20201 ~ 20299 PostgreSQL table error.
	PostgreSQLTableNamingConventionMismatch     Code = 20201
	PostgreSQLTableNoFK                         Code = 20202
	PostgreSQLTableHasFK                        Code = 20203
	PostgreSQLTableDropNamingConventionMismatch Code = 20204
	PostgreSQLTableCommentTooLong               Code = 20205
	PostgreSQLTableCommentRequired              Code = 20206
	PostgreSQLTableCommentConventionMismatch    Code = 20207
	PostgreSQLTableDisallowPartition            Code = 20208
	PostgreSQLTableRequirePK                    Code = 20209
	PostgreSQLTableNoPK                         Code = 20210

	// 20301 ~ 20399 PostgreSQL column error.
	PostgreSQLColumnNamingConventionMismatch      Code = 20301
	PostgreSQLColumnRequiredMissing               Code = 20302
	PostgreSQLColumnCannotNull                    Code = 20303
	PostgreSQLColumnDisallowChangingType          Code = 20304
	PostgreSQLColumnRequireDefault                Code = 20305
	PostgreSQLColumnNoDefault                     Code = 20306
	PostgreSQLColumnDefaultDisallowVolatile       Code = 20307
	PostgreSQLColumnCommentTooLong                Code = 20308
	PostgreSQLColumnCommentRequired               Code = 20309
	PostgreSQLColumnCommentConventionMismatch     Code = 20310
	PostgreSQLDisallowedColumnType                Code = 20311
	PostgreSQLColumnExceedsMaximumCharacterLength Code = 20312

	// 20401 ~ 20499 PostgreSQL index error.
	PostgreSQLIndexNamingConventionMismatch      Code = 20401
	PostgreSQLPrimaryKeyNamingConventionMismatch Code = 20402
	PostgreSQLUniqueKeyNamingConventionMismatch  Code = 20403
	PostgreSQLForeignKeyNamingConventionMismatch Code = 20404
	PostgreSQLIndexNoDuplicateColumn             Code = 20405
	PostgreSQLIndexCountExceedsLimit             Code = 20406
	PostgreSQLIndexTotalNumberExceedsLimit       Code = 20407
	PostgreSQLPrimaryKeyTypeAllowlist            Code = 20408
	PostgreSQLIndexConcurrently                  Code = 20409

	// 20501 ~ 20599 PostgreSQL database error.

	// 20601 ~ 20699 PostgreSQL statement error.
	PostgreSQLNoSelectAll                              Code = 20601
	PostgreSQLInsertRowLimitExceeds                    Code = 20602
	PostgreSQLInsertMustSpecifyColumn                  Code = 20603
	PostgreSQLInsertDisallowOrderByRand                Code = 20604
	PostgreSQLDisallowRemoveTblCascade                 Code = 20605
	PostgreSQLDisallowOnDelCascade                     Code = 20606
	PostgreSQLDisallowCommit                           Code = 20607
	PostgreSQLMergeAlterTable                          Code = 20608
	PostgreSQLStatementAffectedRowLimit                Code = 20609
	PostgreSQLStatementDMLDryRun                       Code = 20610
	PostgreSQLDisallowAddColumnWithDefault             Code = 20611
	PostgreSQLAddCheckNotValid                         Code = 20612
	PostgreSQLAddFKNotValid                            Code = 20613
	PostgreSQLDisallowAddNotNull                       Code = 20614
	PostgreSQLStatementCreateSpecifySchema             Code = 20615
	PostgreSQLStatementCheckSetRoleVariable            Code = 20616
	PostgreSQLStatementDisallowMixInDDL                Code = 20617
	PostgreSQLStatementDisallowMixInDML                Code = 20618
	PostgreSQLStatementMaximumLimitValue               Code = 20619
	PostgreSQLStatementWhereRequirementForSelect       Code = 20620
	PostgreSQLStatementWhereRequirementForUpdateDelete Code = 20621
	PostgreSQLStatementNoLeadingWildcardLike           Code = 20622
	PostgreSQLStatementObjectOwnerCheck                Code = 20623
	PostgreSQLNonTransactional                         Code = 20624

	// 20701 ~ 20799 PostgreSQL system error.
	PostgreSQLNamingFullyQualifiedObjectName Code = 20701
	PostgreSQLEncodingAllowlistViolation     Code = 20702
	PostgreSQLCollationAllowlistViolation    Code = 20703
	PostgreSQLMigrationHasRiskySQLStatement  Code = 20704
	PostgreSQLCommentTooLong                 Code = 20705
	PostgreSQLCommentEmpty                   Code = 20706
	PostgreSQLCommentMissingClassification   Code = 20707

	// 30001 ~ 39999 Oracle error.

	// 30001 ~ 30099 Oracle general error.

	// 30101 ~ 30199 Oracle connection error.

	// 30201 ~ 30299 Oracle table error.
	OracleTableNamingConventionMismatch  Code = 30201
	OracleTableNamingNoKeyword           Code = 30202
	OracleTableNoFK                      Code = 30203
	OracleTableHasFK                     Code = 30204
	OracleTableCommentTooLong            Code = 30205
	OracleTableCommentRequired           Code = 30206
	OracleTableCommentConventionMismatch Code = 30207
	OracleTableRequirePK                 Code = 30208
	OracleTableNoPK                      Code = 30209

	// 30301 ~ 30399 Oracle column error.
	OracleColumnRequiredMissing               Code = 30301
	OracleColumnCannotNull                    Code = 30302
	OracleColumnRequireDefault                Code = 30303
	OracleColumnNoDefault                     Code = 30304
	OracleAddNotNullColumnRequireDefault      Code = 30305
	OracleColumnCommentTooLong                Code = 30306
	OracleColumnCommentRequired               Code = 30307
	OracleColumnCommentConventionMismatch     Code = 30308
	OracleDisallowedColumnType                Code = 30309
	OracleColumnExceedsMaximumCharacterLength Code = 30310
	OracleColumnExceedsMaximumVarcharLength   Code = 30311

	// 30401 ~ 30499 Oracle index error.
	OracleIndexCountExceedsLimit Code = 30401

	// 30501 ~ 30599 Oracle database error.

	// 30601 ~ 30699 Oracle statement error.
	OracleNoSelectAll                              Code = 30601
	OracleInsertMustSpecifyColumn                  Code = 30602
	OracleStatementDisallowMixInDDL                Code = 30603
	OracleStatementDisallowMixInDML                Code = 30604
	OracleStatementDMLDryRun                       Code = 30605
	OracleStatementWhereRequirementForSelect       Code = 30606
	OracleStatementWhereRequirementForUpdateDelete Code = 30607
	OracleStatementNoLeadingWildcardLike           Code = 30608

	// 30701 ~ 30799 Oracle system error.
	OracleIdentifierNamingNoKeyword Code = 30701
	OracleIdentifierCase            Code = 30702

	// 40001 ~ 49999 MSSQL error.

	// 40001 ~ 40099 MSSQL general error.

	// 40101 ~ 40199 MSSQL connection error.

	// 40201 ~ 40299 MSSQL table error.
	MSSQLTableNamingConventionMismatch     Code = 40201
	MSSQLTableNamingNoKeyword              Code = 40202
	MSSQLTableNoFK                         Code = 40203
	MSSQLTableHasFK                        Code = 40204
	MSSQLTableDropNamingConventionMismatch Code = 40205
	MSSQLTableRequirePK                    Code = 40206
	MSSQLTableNoPK                         Code = 40207
	MSSQLTableDisallowDDL                  Code = 40208
	MSSQLTableDisallowDML                  Code = 40209

	// 40301 ~ 40399 MSSQL column error.
	MSSQLColumnRequiredMissing             Code = 40301
	MSSQLColumnCannotNull                  Code = 40302
	MSSQLDisallowedColumnType              Code = 40303
	MSSQLColumnExceedsMaximumVarcharLength Code = 40304

	// 40401 ~ 40499 MSSQL index error.
	MSSQLIndexNotRedundant Code = 40401

	// 40501 ~ 40599 MSSQL database error.

	// 40601 ~ 40699 MSSQL statement error.
	MSSQLNoSelectAll                                    Code = 40601
	MSSQLStatementDisallowMixInDDL                      Code = 40602
	MSSQLStatementDisallowMixInDML                      Code = 40603
	MSSQLStatementWhereDisallowFunctionsAndCalculations Code = 40604
	MSSQLStatementWhereRequirementForSelect             Code = 40605
	MSSQLStatementWhereRequirementForUpdateDelete       Code = 40606
	MSSQLStatementDisallowCrossDBQueries                Code = 40607

	// 40701 ~ 40799 MSSQL system error.
	MSSQLIdentifierNamingNoKeyword      Code = 40701
	MSSQLMigrationHasRiskySQLStatement  Code = 40702
	MSSQLProcedureDisallowCreateOrAlter Code = 40703
	MSSQLFunctionDisallowCreateOrAlter  Code = 40704

	// 50001 ~ 59999 Snowflake error.

	// 50001 ~ 50099 Snowflake general error.

	// 50101 ~ 50199 Snowflake connection error.

	// 50201 ~ 50299 Snowflake table error.
	SnowflakeTableNamingConventionMismatch     Code = 50201
	SnowflakeTableNamingNoKeyword              Code = 50202
	SnowflakeTableNoFK                         Code = 50203
	SnowflakeTableHasFK                        Code = 50204
	SnowflakeTableDropNamingConventionMismatch Code = 50205
	SnowflakeTableRequirePK                    Code = 50206
	SnowflakeTableNoPK                         Code = 50207

	// 50301 ~ 50399 Snowflake column error.
	SnowflakeColumnRequiredMissing             Code = 50301
	SnowflakeColumnCannotNull                  Code = 50302
	SnowflakeColumnExceedsMaximumVarcharLength Code = 50303

	// 50401 ~ 50499 Snowflake index error.

	// 50501 ~ 50599 Snowflake database error.

	// 50601 ~ 50699 Snowflake statement error.
	SnowflakeNoSelectAll                              Code = 50601
	SnowflakeStatementWhereRequirementForSelect       Code = 50602
	SnowflakeStatementWhereRequirementForUpdateDelete Code = 50603

	// 50701 ~ 50799 Snowflake system error.
	SnowflakeIdentifierNamingNoKeyword     Code = 50701
	SnowflakeIdentifierCase                Code = 50702
	SnowflakeMigrationHasRiskySQLStatement Code = 50703
)

// Int32 returns the int32 representation of the Code.
func (c Code) Int32() int32 {
	return int32(c)
}
