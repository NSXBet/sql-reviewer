package mysql

import (
	"encoding/json"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// MockTableName is the mock table name for tests
const MockTableName = "tech_book"

// MockMySQLDatabase creates a mock MySQL database schema for testing (similar to Bytebase MockMySQLDatabase)
func MockMySQLDatabase() *types.DatabaseSchemaMetadata {
	return &types.DatabaseSchemaMetadata{
		Name: "test",
		Schemas: []*types.SchemaMetadata{
			{
				Name: "",
				Tables: []*types.TableMetadata{
					{
						Name: MockTableName,
						Columns: []*types.ColumnMetadata{
							{
								Name: "id",
								Type: "int",
							},
							{
								Name: "name",
								Type: "varchar(255)",
							},
						},
					},
				},
			},
		},
	}
}

// CreateMockCatalogFinder creates a catalog finder with mock database schema
func CreateMockCatalogFinder() *catalog.Finder {
	mockDB := MockMySQLDatabase()
	return catalog.NewFinder(mockDB, &catalog.FinderContext{
		CheckIntegrity:      true,
		EngineType:          types.Engine_MYSQL,
		IgnoreCaseSensitive: true,
	})
}

// CreateMockDatabaseForTableRequirePK creates a mock database schema for table require PK tests
func CreateMockDatabaseForTableRequirePK() *types.DatabaseSchemaMetadata {
	return &types.DatabaseSchemaMetadata{
		Name:         "test",
		CharacterSet: "",
		Collation:    "",
		Schemas: []*types.SchemaMetadata{
			{
				Name: "",
				Tables: []*types.TableMetadata{
					{
						Name: "tech_book",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Type:     "int",
								Nullable: false,
								Position: 1,
							},
							{
								Name:     "name",
								Type:     "varchar",
								Nullable: false,
								Position: 2,
							},
						},
						Indexes: []*types.IndexMetadata{
							{
								Name:        "PRIMARY",
								Expressions: []string{"id", "name"},
								Type:        "BTREE",
								Unique:      true,
								Primary:     true,
							},
						},
					},
				},
				Views: []*types.ViewMetadata{},
			},
		},
	}
}

// SetDefaultSQLReviewRulePayload sets the default payload for rules that require them.
func SetDefaultSQLReviewRulePayload(ruleType advisor.SQLReviewRuleType) (map[string]interface{}, error) {
	var payload interface{}

	switch ruleType {
	case advisor.SchemaRuleStatementAffectedRowLimit:
		payload = advisor.NumberTypeRulePayload{
			Number: 5,
		}
	case advisor.SchemaRuleCharsetAllowlist:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"utf8mb4"},
		}
	case advisor.SchemaRuleCollationAllowlist:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"utf8mb4_0900_ai_ci"},
		}
	case advisor.SchemaRuleStatementWhereMaximumLogicalOperatorCount:
		payload = advisor.NumberTypeRulePayload{
			Number: 2,
		}
	case advisor.SchemaRuleStatementInsertRowLimit:
		payload = advisor.NumberTypeRulePayload{
			Number: 5,
		}
	case advisor.SchemaRuleIndexTotalNumberLimit:
		payload = advisor.NumberTypeRulePayload{
			Number: 5,
		}
	case advisor.SchemaRuleTableDisallowDDL:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"audit_log", "system_config"},
		}
	case advisor.SchemaRuleTableDisallowDML:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"audit_log", "system_config"},
		}
	case advisor.SchemaRuleTableTextFieldsTotalLength:
		payload = advisor.NumberTypeRulePayload{
			Number: 10000, // Max 10KB total text length
		}
	case advisor.SchemaRuleTableLimitSize:
		payload = advisor.NumberTypeRulePayload{
			Number: 1000000, // Max 1M rows
		}
	case advisor.SchemaRuleStatementMaximumJoinTableCount:
		payload = advisor.NumberTypeRulePayload{
			Number: 2, // Max 2 joins
		}
	case advisor.SchemaRuleStatementMaximumStatementsInTransaction:
		payload = advisor.NumberTypeRulePayload{
			Number: 3, // Max 3 statements per transaction
		}
	case advisor.SchemaRuleStatementQueryMinumumPlanLevel:
		payload = advisor.StringTypeRulePayload{
			String: "INDEX", // Minimum plan level INDEX
		}
	case advisor.SchemaRuleTableNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^[a-z]+(_[a-z]+)*$", // Default snake_case pattern
		}
	case advisor.SchemaRuleColumnMaximumCharacterLength:
		payload = advisor.NumberTypeRulePayload{
			Number: 20, // Default maximum length
		}
	case advisor.SchemaRuleColumnMaximumVarcharLength:
		payload = advisor.NumberTypeRulePayload{
			Number: 2560, // Default maximum length
		}
	case advisor.SchemaRuleColumnTypeDisallowList:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"JSON"}, // Default disallowed types
		}
	case advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"int", "bigint"}, // Default allowed types
		}
	case advisor.SchemaRuleIndexTypeAllowList:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"BTREE"}, // Default allowed types
		}
	case advisor.SchemaRuleAutoIncrementColumnNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^id$", // Default pattern from test data
		}
	case advisor.SchemaRuleFKNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^$|^fk_{{referencing_table}}_{{referencing_column}}_{{referenced_table}}_{{referenced_column}}$", // Default FK naming pattern
		}
	case advisor.SchemaRuleIDXNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^$|^idx_{{table}}_{{column_list}}$", // Default index naming pattern
		}
	case advisor.SchemaRuleUKNaming:
		payload = advisor.StringTypeRulePayload{
			String: "^$|^uk_{{table}}_{{column_list}}$", // Default unique key naming pattern
		}
	case advisor.SchemaRuleFunctionDisallowList:
		payload = advisor.StringArrayTypeRulePayload{
			List: []string{"RAND", "UUID", "SLEEP"}, // Default disallowed functions
		}
	case advisor.SchemaRuleTableDropNamingConvention:
		payload = advisor.StringTypeRulePayload{
			String: "_delete$", // Default drop naming pattern
		}
	case advisor.SchemaRuleColumnAutoIncrementInitialValue:
		payload = advisor.NumberTypeRulePayload{
			Number: 20, // Default initial value
		}
	case advisor.SchemaRuleStatementMaximumLimitValue:
		payload = advisor.NumberTypeRulePayload{
			Number: 1000, // Default maximum limit value
		}
	default:
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

// CreateMockDatabaseForIndexTotalNumberLimit creates a mock database schema for index total number limit tests
// This mock contains only the baseline state - no pre-created table t, and tech_book with 3 existing indexes
func CreateMockDatabaseForIndexTotalNumberLimit() *types.DatabaseSchemaMetadata {
	return &types.DatabaseSchemaMetadata{
		Name:         "test",
		CharacterSet: "",
		Collation:    "",
		Schemas: []*types.SchemaMetadata{
			{
				Name: "",
				Tables: []*types.TableMetadata{
					// tech_book table with 3 existing indexes (baseline state)
					{
						Name: "tech_book",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Type:     "int",
								Nullable: false,
								Position: 1,
							},
							{
								Name:     "name",
								Type:     "varchar",
								Nullable: false,
								Position: 2,
							},
						},
						Indexes: []*types.IndexMetadata{
							{
								Name:        "PRIMARY",
								Expressions: []string{"id"},
								Type:        "BTREE",
								Unique:      true,
								Primary:     true,
							},
							{
								Name:        "idx_existing_1",
								Expressions: []string{"name"},
								Type:        "BTREE",
								Unique:      false,
								Primary:     false,
							},
							{
								Name:        "idx_existing_2",
								Expressions: []string{"id", "name"},
								Type:        "BTREE",
								Unique:      false,
								Primary:     false,
							},
						},
					},
					// Note: table t is not pre-created - it will be created by the test statements
				},
				Views: []*types.ViewMetadata{},
			},
		},
	}
}
