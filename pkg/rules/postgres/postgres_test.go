package postgres

import (
	"context"
	"encoding/json"
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
	advisor.SchemaRuleColumnNotNull:            true, // PRIMARY KEY USING INDEX
	advisor.SchemaRuleTableRequirePK:           true, // DROP CONSTRAINT/COLUMN checks
	advisor.SchemaRuleIndexTotalNumberLimit:    true, // Count existing indexes
	advisor.SchemaRulePKNaming:                 true, // PRIMARY KEY USING INDEX
	advisor.SchemaRuleUKNaming:                 true, // UNIQUE USING INDEX
	advisor.SchemaRuleIDXNaming:                true, // ALTER INDEX RENAME
	advisor.SchemaRuleFullyQualifiedObjectName: true, // SELECT statement checks
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
						require.Equal(t, want.Code, got.Code, "Advice %d: code mismatch", j)
					}

					// Compare title
					require.Equal(t, want.Title, got.Title, "Advice %d: title mismatch", j)

					// Compare content
					require.Equal(t, want.Content, got.Content, "Advice %d: content mismatch", j)

					// Compare position if specified
					if want.StartPosition != nil {
						require.NotNil(t, got.StartPosition, "Advice %d: expected start position", j)
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
	var payload interface{}

	switch rule {
	case advisor.SchemaRuleTableNaming,
		advisor.SchemaRuleColumnNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^[a-z]+(_[a-z]+)*$",
		}
	case advisor.SchemaRulePKNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^[a-z]+(_[a-z]+)*_pkey$",
		}
	case advisor.SchemaRuleUKNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^[a-z]+(_[a-z]+)*_uk$",
		}
	case advisor.SchemaRuleFKNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^[a-z]+(_[a-z]+)*_fkey$",
		}
	case advisor.SchemaRuleIDXNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^[a-z]+(_[a-z]+)*_idx$",
		}
	case advisor.SchemaRuleTableDropNamingConvention:
		payload = advisor.StringTypeRulePayload{
			String: "^[a-z]+(_[a-z]+)*_del$",
		}
	case advisor.SchemaRuleRequiredColumn:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"id", "created_at", "updated_at"},
		}
	case advisor.SchemaRuleColumnTypeDisallowList:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"text", "json"},
		}
	case advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"int", "bigint"},
		}
	case advisor.SchemaRuleCharsetAllowlist:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"utf8", "utf8mb4"},
		}
	case advisor.SchemaRuleCollationAllowlist:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"utf8mb4_0900_ai_ci", "utf8mb4_general_ci"},
		}
	case advisor.SchemaRuleColumnMaximumCharacterLength,
		advisor.SchemaRuleColumnCommentConvention,
		advisor.SchemaRuleTableCommentConvention:
		payload = advisor.NumberTypeRulePayload{
			Number: 20,
		}
	case advisor.SchemaRuleIndexKeyNumberLimit,
		advisor.SchemaRuleIndexTotalNumberLimit:
		payload = advisor.NumberTypeRulePayload{
			Number: 5,
		}
	case advisor.SchemaRuleStatementInsertRowLimit:
		payload = advisor.NumberTypeRulePayload{
			Number: 100,
		}
	case advisor.SchemaRuleStatementMaximumLimitValue:
		payload = advisor.NumberTypeRulePayload{
			Number: 1000,
		}
	case advisor.SchemaRuleStatementAffectedRowLimit:
		payload = advisor.NumberTypeRulePayload{
			Number: 1000,
		}
	default:
		// Rules without payload
		return nil, nil
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// createMockDatabase creates a mock database schema for testing.
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
				},
			},
		},
	}
}
