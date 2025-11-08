package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/catalog"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestCase represents a single test case from the YAML file.
type TestCase struct {
	Statement  string         `yaml:"statement"`
	ChangeType int            `yaml:"changeType"`
	Want       []types.Advice `yaml:"want"`
}

// catalogWrapper wraps a catalog.Finder to implement the catalogInterface.
type catalogWrapper struct {
	finder *catalog.Finder
}

func (c *catalogWrapper) GetFinder() *catalog.Finder {
	return c.finder
}

// TestPostgreSQLRules runs all PostgreSQL rules against their testdata files.
func TestPostgreSQLRules(t *testing.T) {
	rules := []advisor.SQLReviewRuleType{
		advisor.SchemaRuleCharsetAllowlist,
		advisor.SchemaRuleCollationAllowlist,
		advisor.SchemaRuleColumnAutoIncrementMustInteger,
		advisor.SchemaRuleColumnAutoIncrementMustUnsigned,
		advisor.SchemaRuleColumnCommentConvention,
		advisor.SchemaRuleColumnDefaultDisallowVolatile,
		advisor.SchemaRuleColumnDisallowChangeType,
		advisor.SchemaRuleColumnMaximumCharacterLength,
		advisor.SchemaRuleColumnNaming,
		advisor.SchemaRuleColumnNotNull,
		advisor.SchemaRuleColumnRequireDefault,
		advisor.SchemaRuleColumnSetDefaultForNotNull,
		advisor.SchemaRuleColumnTypeDisallowList,
		advisor.SchemaRuleCreateIndexConcurrently,
		advisor.SchemaRuleFKNaming,
		advisor.SchemaRuleFullyQualifiedObjectName,
		advisor.SchemaRuleIDXNaming,
		advisor.SchemaRuleIndexKeyNumberLimit,
		advisor.SchemaRuleIndexNoDuplicateColumn,
		advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist,
		advisor.SchemaRuleIndexTotalNumberLimit,
		advisor.SchemaRulePKNaming,
		advisor.SchemaRuleRequiredColumn,
		advisor.SchemaRuleStatementAddCheckNotValid,
		advisor.SchemaRuleStatementAddFKNotValid,
		advisor.SchemaRuleStatementAffectedRowLimit,
		advisor.SchemaRuleStatementCheckSetRoleVariable,
		advisor.SchemaRuleStatementCreateSpecifySchema,
		advisor.SchemaRuleStatementDisallowAddColumnWithDefault,
		advisor.SchemaRuleStatementDisallowAddNotNull,
		advisor.SchemaRuleStatementDisallowCommit,
		advisor.SchemaRuleStatementDisallowMixInDDL,
		advisor.SchemaRuleStatementDisallowMixInDML,
		advisor.SchemaRuleStatementDisallowOnDelCascade,
		advisor.SchemaRuleStatementDisallowRemoveTblCascade,
		advisor.SchemaRuleStatementDMLDryRun,
		advisor.SchemaRuleStatementInsertDisallowOrderByRand,
		advisor.SchemaRuleStatementInsertMustSpecifyColumn,
		advisor.SchemaRuleStatementInsertRowLimit,
		advisor.SchemaRuleStatementMaximumLimitValue,
		advisor.SchemaRuleStatementMergeAlterTable,
		advisor.SchemaRuleStatementNoLeadingWildcardLike,
		advisor.SchemaRuleStatementNonTransactional,
		advisor.SchemaRuleStatementNoSelectAll,
		advisor.SchemaRuleStatementObjectOwnerCheck,
		advisor.SchemaRuleStatementPriorBackupCheck,
		advisor.SchemaRuleStatementRequireWhereForSelect,
		advisor.SchemaRuleStatementRequireWhereForUpdateDelete,
		advisor.SchemaRuleTableCommentConvention,
		advisor.SchemaRuleTableDisallowPartition,
		advisor.SchemaRuleTableDropNamingConvention,
		advisor.SchemaRuleTableNaming,
		advisor.SchemaRuleTableNoFK,
		advisor.SchemaRuleTableRequirePK,
		advisor.SchemaRuleUKNaming,
	}

	for _, rule := range rules {
		needMetadata := rulesNeedingMetadata[rule]
		runPostgreSQLRuleTest(t, rule, needMetadata)
	}
}

// rulesNeedingMetadata specifies which rules need catalog metadata for testing.
// These rules interact with existing database schema (e.g., checking existing indexes,
// primary keys, or object ownership).
var rulesNeedingMetadata = map[advisor.SQLReviewRuleType]bool{
	advisor.SchemaRuleColumnNotNull:             true, // PRIMARY KEY USING INDEX
	advisor.SchemaRuleTableRequirePK:            true, // DROP CONSTRAINT/COLUMN checks
	advisor.SchemaRuleIndexTotalNumberLimit:     true, // Count existing indexes
	advisor.SchemaRulePKNaming:                  true, // PRIMARY KEY USING INDEX
	advisor.SchemaRuleUKNaming:                  true, // UNIQUE USING INDEX
	advisor.SchemaRuleIDXNaming:                 true, // ALTER INDEX RENAME
	advisor.SchemaRuleFullyQualifiedObjectName:  true, // SELECT statement checks
	advisor.SchemaRuleStatementObjectOwnerCheck: true, // Ownership checks
}

// runPostgreSQLRuleTest runs a single PostgreSQL rule against its testdata file.
func runPostgreSQLRuleTest(t *testing.T, rule advisor.SQLReviewRuleType, needMetadata bool) {
	t.Helper()

	// Convert rule type to filename (e.g., "naming.column" -> "naming_column")
	fileName := strings.Map(func(r rune) rune {
		switch r {
		case '.', '-':
			return '_'
		default:
			return r
		}
	}, string(rule))

	// Load test cases from YAML file
	testDataFile := filepath.Join("testdata", fileName+".yaml")
	testCases, err := loadTestCases(testDataFile)
	if err != nil {
		t.Skipf("Skipping %s: no testdata file found (%v)", rule, err)
		return
	}

	for i, tc := range testCases {
		testName := fmt.Sprintf("%s[%d]", rule, i)
		if len(tc.Statement) > 50 {
			testName = fmt.Sprintf("%s: %s...", testName, tc.Statement[:50])
		} else {
			testName = fmt.Sprintf("%s: %s", testName, tc.Statement)
		}

		t.Run(testName, func(t *testing.T) {
			// Parse the SQL
			parseResult, err := pgparser.ParsePostgreSQL(tc.Statement)
			require.NoError(t, err, "Failed to parse SQL: %s", tc.Statement)

			// Create catalog finder with metadata if needed
			var catalogIface interface {
				GetFinder() *catalog.Finder
			}
			if needMetadata {
				// Create a mock database schema for testing
				mockDatabase := createMockDatabase()
				finder := catalog.NewFinder(
					mockDatabase,
					&catalog.FinderContext{
						CheckIntegrity: true,
						EngineType:     types.Engine_POSTGRES,
					},
				)

				// Walk through the parse tree to build catalog
				err = finder.WalkThrough(parseResult)
				require.NoError(t, err, "Failed to walk through catalog: %s", tc.Statement)

				// Wrap finder to implement catalogInterface
				catalogIface = &catalogWrapper{finder: finder}
			}

			// Get default payload for the rule
			payload, err := getDefaultPayload(rule)
			if err != nil {
				// Use empty payload for rules without defaults
				payload = map[string]interface{}{}
			}

			// Create the rule
			sqlRule := &types.SQLReviewRule{
				Type:    string(rule),
				Level:   types.SQLReviewRuleLevel_WARNING,
				Payload: payload,
			}

			// Create check context
			checkCtx := advisor.Context{
				DBType:     types.Engine_POSTGRES,
				Statements: tc.Statement,
				Rule:       sqlRule,
				AST:        parseResult,
				ChangeType: types.PlanCheckRunConfig_ChangeDatabaseType(tc.ChangeType),
				Catalog:    catalogIface,
			}

			// Run the advisor
			ctx := context.Background()
			advices, err := advisor.Check(ctx, types.Engine_POSTGRES, advisor.Type(rule), checkCtx)
			require.NoError(t, err, "Advisor check failed")

			// Compare results
			if len(tc.Want) == 0 {
				// Expect no violations
				require.Empty(t, advices, "Expected no violations but got: %+v", advices)
			} else {
				// Expect specific violations
				require.Len(t, advices, len(tc.Want), "Expected %d violations but got %d", len(tc.Want), len(advices))

				for j, want := range tc.Want {
					got := advices[j]

					// Compare status
					require.Equal(t, want.Status, got.Status, "Advice %d: status mismatch", j)

					// Compare code if specified
					if want.Code != 0 {
						if want.Code != got.Code {
							t.Logf("DEBUG: Expected code %d but got %d. Advice content: %s", want.Code, got.Code, got.Content)
						}
						require.Equal(t, want.Code, got.Code, "Advice %d: code mismatch", j)
					}

					// Compare title
					require.Equal(t, want.Title, got.Title, "Advice %d: title mismatch", j)

					// Compare content
					require.Equal(t, want.Content, got.Content, "Advice %d: content mismatch", j)

					// Compare position if specified
					if want.StartPosition != nil {
						require.NotNil(t, got.StartPosition, "Advice %d: expected start position", j)
						if want.StartPosition.Line != got.StartPosition.Line {
							t.Logf("DEBUG Line mismatch for advice %d: want line %d, got line %d. Content: %s",
								j, want.StartPosition.Line, got.StartPosition.Line, got.Content)
						}
						require.Equal(t, want.StartPosition.Line, got.StartPosition.Line, "Advice %d: line mismatch", j)
					}
				}
			}
		})
	}
}

// loadTestCases loads test cases from a YAML file.
func loadTestCases(filename string) ([]TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file %s: %w", filename, err)
	}

	var testCases []TestCase
	if err := yaml.Unmarshal(data, &testCases); err != nil {
		return nil, fmt.Errorf("failed to unmarshal test cases from %s: %w", filename, err)
	}

	return testCases, nil
}

// getDefaultPayload returns the default payload for a PostgreSQL rule.
func getDefaultPayload(rule advisor.SQLReviewRuleType) (map[string]interface{}, error) {
	switch rule {
	case advisor.SchemaRuleTableNaming:
		return map[string]interface{}{
			"format":    "^[a-z]+(_[a-z]+)*$",
			"maxLength": 64,
		}, nil
	case advisor.SchemaRuleColumnNaming:
		return map[string]interface{}{
			"format":    "^[a-z]+(_[a-z]+)*$",
			"maxLength": 64,
		}, nil
	case advisor.SchemaRulePKNaming:
		return map[string]interface{}{
			"format":       "^$|^pk_{{table}}_{{column_list}}$",
			"maxLength":    64,
			"templateList": []string{"table", "column_list"},
		}, nil
	case advisor.SchemaRuleUKNaming:
		return map[string]interface{}{
			"format":       "^$|^uk_{{table}}_{{column_list}}$",
			"maxLength":    64,
			"templateList": []string{"table", "column_list"},
		}, nil
	case advisor.SchemaRuleFKNaming:
		return map[string]interface{}{
			"format":       "^$|^fk_{{referencing_table}}_{{referencing_column}}_{{referenced_table}}_{{referenced_column}}$",
			"maxLength":    64,
			"templateList": []string{"referencing_table", "referencing_column", "referenced_table", "referenced_column"},
		}, nil
	case advisor.SchemaRuleIDXNaming:
		return map[string]interface{}{
			"format":       "^$|^idx_{{table}}_{{column_list}}$",
			"maxLength":    64,
			"templateList": []string{"table", "column_list"},
		}, nil
	case advisor.SchemaRuleTableDropNamingConvention:
		return map[string]interface{}{
			"format":    "_delete$",
			"maxLength": 64,
		}, nil
	case advisor.SchemaRuleRequiredColumn:
		return map[string]interface{}{
			"list": []string{"created_ts", "creator_id", "updated_ts", "updater_id"},
		}, nil
	case advisor.SchemaRuleColumnTypeDisallowList:
		return map[string]interface{}{
			"list": []string{"text", "json"},
		}, nil
	case advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist:
		return map[string]interface{}{
			"list": []string{"int", "bigint"},
		}, nil
	case advisor.SchemaRuleCharsetAllowlist:
		return map[string]interface{}{
			"list": []string{"utf8", "utf8mb4"},
		}, nil
	case advisor.SchemaRuleCollationAllowlist:
		return map[string]interface{}{
			"list": []string{"utf8mb4_0900_ai_ci", "utf8mb4_general_ci"},
		}, nil
	case advisor.SchemaRuleColumnMaximumCharacterLength:
		return map[string]interface{}{
			"number": 20,
		}, nil
	case advisor.SchemaRuleColumnCommentConvention:
		return map[string]interface{}{
			"number": 20,
		}, nil
	case advisor.SchemaRuleTableCommentConvention:
		return map[string]interface{}{
			"number": 20,
		}, nil
	case advisor.SchemaRuleIndexKeyNumberLimit:
		return map[string]interface{}{
			"number": 5,
		}, nil
	case advisor.SchemaRuleIndexTotalNumberLimit:
		return map[string]interface{}{
			"number": 5,
		}, nil
	case advisor.SchemaRuleStatementInsertRowLimit:
		return map[string]interface{}{
			"number": 100,
		}, nil
	case advisor.SchemaRuleStatementMaximumLimitValue:
		return map[string]interface{}{
			"number": 1000,
		}, nil
	case advisor.SchemaRuleStatementAffectedRowLimit:
		return map[string]interface{}{
			"number": 1000,
		}, nil
	default:
		// Rules without payload
		return nil, nil
	}
}

// createMockDatabase creates a mock database schema for testing.
// Following Bytebase's pattern: tech_book has existing constraints with WRONG names
// so tests can rename them or add new ones with correct names.
func createMockDatabase() *types.DatabaseSchemaMetadata {
	return &types.DatabaseSchemaMetadata{
		Name:         "test_db",
		CharacterSet: "UTF8",
		Collation:    "en_US.UTF-8",
		Schemas: []*types.SchemaMetadata{
			{
				Name: "public",
				Tables: []*types.TableMetadata{
					{
						Name: "users",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Position: 1,
								Type:     "integer",
								Nullable: false,
							},
							{
								Name:     "name",
								Position: 2,
								Type:     "varchar(100)",
								Nullable: true,
							},
							{
								Name:     "email",
								Position: 3,
								Type:     "varchar(255)",
								Nullable: false,
							},
						},
						Indexes: []*types.IndexMetadata{
							{
								Name:   "users_pkey",
								Type:   "PRIMARY KEY",
								Unique: true,
								Expressions: []string{
									"id",
								},
								Primary: true,
							},
							{
								Name:   "users_email_idx",
								Type:   "INDEX",
								Unique: false,
								Expressions: []string{
									"email",
								},
							},
						},
					},
					{
						Name: "orders",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Position: 1,
								Type:     "integer",
								Nullable: false,
							},
							{
								Name:     "user_id",
								Position: 2,
								Type:     "integer",
								Nullable: false,
							},
							{
								Name:     "amount",
								Position: 3,
								Type:     "numeric(10,2)",
								Nullable: false,
							},
						},
						Indexes: []*types.IndexMetadata{
							{
								Name:   "orders_pkey",
								Type:   "PRIMARY KEY",
								Unique: true,
								Expressions: []string{
									"id",
								},
								Primary: true,
							},
						},
					},
					{
						Name: "tech_book",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Position: 1,
								Type:     "integer",
								Nullable: false,
							},
							{
								Name:     "name",
								Position: 2,
								Type:     "varchar(255)",
								Nullable: false,
							},
						},
						// Existing constraints with WRONG names for testing rename/add
						Indexes: []*types.IndexMetadata{
							{
								Name:   "old_pk",
								Type:   "PRIMARY KEY",
								Unique: true,
								Expressions: []string{
									"id",
									"name",
								},
								Primary: true,
							},
							{
								Name:   "old_uk",
								Type:   "UNIQUE",
								Unique: true,
								Expressions: []string{
									"id",
									"name",
								},
							},
							{
								Name:   "old_index",
								Type:   "INDEX",
								Unique: false,
								Expressions: []string{
									"id",
									"name",
								},
							},
						},
					},
					{
						Name: "author",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Position: 1,
								Type:     "integer",
								Nullable: false,
							},
							{
								Name:     "name",
								Position: 2,
								Type:     "varchar(255)",
								Nullable: false,
							},
						},
						Indexes: []*types.IndexMetadata{
							{
								Name:   "author_pkey",
								Type:   "PRIMARY KEY",
								Unique: true,
								Expressions: []string{
									"id",
								},
								Primary: true,
							},
						},
					},
				},
			},
		},
	}
}
