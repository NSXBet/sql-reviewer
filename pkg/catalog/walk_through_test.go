package catalog

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

type testCase struct {
	Statement             string                 `yaml:"statement"`
	IgnoreCaseSensitive   bool                   `yaml:"ignore_case_sensitive"`
	Want                  string                 `yaml:"want"`
	Err                   *testCaseErrorMetadata `yaml:"err"`
}

type testCaseErrorMetadata struct {
	Type    int    `yaml:"type"`
	Content string `yaml:"content"`
	Line    int    `yaml:"line"`
	Payload any    `yaml:"payload"`
}

func TestMySQLWalkThrough(t *testing.T) {
	testCases := getMySQLWalkThroughTestCases(t)

	for i, tc := range testCases {
		t.Run(tc.Statement, func(t *testing.T) {
			// Create database metadata
			database := &types.DatabaseSchemaMetadata{
				Name:         "test",
				CharacterSet: "",
				Collation:    "",
				Schemas: []*types.SchemaMetadata{
					{
						Name:   "",
						Tables: []*types.TableMetadata{},
						Views:  []*types.ViewMetadata{},
					},
				},
			}

			// Create finder context
			ctx := &FinderContext{
				CheckIntegrity:      true,
				EngineType:          types.Engine_MYSQL,
				IgnoreCaseSensitive: !tc.IgnoreCaseSensitive,
			}

			// Create finder
			finder := NewFinder(database, ctx)

			// Parse SQL statements
			stmts, err := mysqlparser.ParseMySQL(tc.Statement)
			require.NoError(t, err)

			// Convert to format expected by WalkThrough
			var parseResults []*mysqlParseResult
			for _, stmt := range stmts {
				parseResults = append(parseResults, &mysqlParseResult{
					Tree:     stmt.Tree,
					BaseLine: stmt.BaseLine,
				})
			}

			// Execute walk through
			walkErr := finder.WalkThrough(parseResults)

			if tc.Err != nil {
				// Expecting an error
				require.Error(t, walkErr, "Test case %d should have error", i)
				
				walkThroughErr, ok := walkErr.(*WalkThroughError)
				require.True(t, ok, "Error should be WalkThroughError")
				require.Equal(t, WalkThroughErrorType(tc.Err.Type), walkThroughErr.Type)
				require.Equal(t, tc.Err.Content, walkThroughErr.Content)
				if tc.Err.Line != 0 {
					require.Equal(t, tc.Err.Line, walkThroughErr.Line)
				}
			} else {
				// Expecting success
				require.NoError(t, walkErr, "Test case %d should not have error", i)

				if tc.Want != "" {
					// Compare result
					actualBytes, err := json.MarshalIndent(simplifyDatabaseMetadata(finder.Final), "", "  ")
					require.NoError(t, err)

					var wantObj, actualObj interface{}
					err = json.Unmarshal([]byte(tc.Want), &wantObj)
					require.NoError(t, err)
					err = json.Unmarshal(actualBytes, &actualObj)
					require.NoError(t, err)

					require.Equal(t, wantObj, actualObj, "Test case %d result mismatch", i)
				}
			}
		})
	}
}

func getMySQLWalkThroughTestCases(t *testing.T) []testCase {
	yamlData, err := os.ReadFile("./test/mysql_walk_through.yaml")
	require.NoError(t, err)

	var testCases []testCase
	err = yaml.Unmarshal(yamlData, &testCases)
	require.NoError(t, err)
	return testCases
}

// simplifyDatabaseMetadata converts DatabaseState to a simplified structure for testing
func simplifyDatabaseMetadata(state *DatabaseState) map[string]interface{} {
	result := map[string]interface{}{
		"name": state.name,
	}

	// Only include characterSet and collation if they are set (even if default)
	if state.characterSet != "" {
		result["characterSet"] = state.characterSet
	}
	if state.collation != "" {
		result["collation"] = state.collation
	}

	var schemas []map[string]interface{}
	for _, schema := range state.schemaSet {
		schemaMap := map[string]interface{}{}

		if len(schema.tableSet) > 0 {
			var tables []map[string]interface{}
			for _, table := range schema.tableSet {
				tableMap := map[string]interface{}{
					"name": table.name,
				}

				// Add columns
				if len(table.columnSet) > 0 {
					var columns []map[string]interface{}
					for _, column := range table.columnSet {
						columnMap := map[string]interface{}{
							"name":     column.name,
							"position": *column.position,
						}
						if column.nullable != nil && *column.nullable {
							columnMap["nullable"] = true
						}
						if column.columnType != nil && *column.columnType != "" {
							columnMap["type"] = *column.columnType
						}
						if column.defaultValue != nil && *column.defaultValue != "" {
							columnMap["default"] = *column.defaultValue
						}
						if column.comment != nil && *column.comment != "" {
							columnMap["comment"] = *column.comment
						}
						if column.characterSet != nil && *column.characterSet != "" {
							columnMap["characterSet"] = *column.characterSet
						}
						if column.collation != nil && *column.collation != "" {
							columnMap["collation"] = *column.collation
						}
						columns = append(columns, columnMap)
					}
					tableMap["columns"] = columns
				}

				// Add indexes
				if len(table.indexSet) > 0 {
					var indexes []map[string]interface{}
					
					// Sort indexes by name to ensure consistent ordering
					var indexNames []string
					for indexName := range table.indexSet {
						indexNames = append(indexNames, indexName)
					}
					sort.Strings(indexNames)
					
					for _, indexName := range indexNames {
						index := table.indexSet[indexName]
						indexMap := map[string]interface{}{
							"name":        index.name,
							"expressions": index.expressionList,
						}
						if index.indexType != nil && *index.indexType != "" {
							indexMap["type"] = *index.indexType
						}
						if index.unique != nil && *index.unique {
							indexMap["unique"] = true
						}
						if index.primary != nil && *index.primary {
							indexMap["primary"] = true
						}
						if index.visible != nil && *index.visible {
							indexMap["visible"] = true
						}
						indexes = append(indexes, indexMap)
					}
					tableMap["indexes"] = indexes
				}

				// Add table properties
				if table.engine != nil && *table.engine != "" {
					tableMap["engine"] = *table.engine
				}
				if table.collation != nil && *table.collation != "" {
					tableMap["collation"] = *table.collation
				}
				if table.comment != nil && *table.comment != "" {
					tableMap["comment"] = *table.comment
				}

				tables = append(tables, tableMap)
			}
			schemaMap["tables"] = tables
		}

		schemas = append(schemas, schemaMap)
	}
	result["schemas"] = schemas

	return result
}