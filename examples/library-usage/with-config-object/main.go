// Package main demonstrates using SQL reviewer with programmatic config objects.
//
// This example shows how to:
//   - Create config objects programmatically (no YAML files needed)
//   - Configure backward compatibility rules for MySQL and PostgreSQL
//   - Use different payload types (string, number, array, boolean)
//   - Demonstrate backward compatible payload formats
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/config"
	"github.com/nsxbet/sql-reviewer/pkg/reviewer"
	"github.com/nsxbet/sql-reviewer/pkg/types"

	// Import rule packages to register all advisors
	_ "github.com/nsxbet/sql-reviewer/pkg/rules/mysql"
	_ "github.com/nsxbet/sql-reviewer/pkg/rules/postgres"
)

func main() {
	fmt.Println("SQL Reviewer - Config Object Example")
	fmt.Println("=====================================")

	// Example 1: MySQL with backward compatibility rules
	fmt.Println("Example 1: MySQL Backward Compatibility")
	fmt.Println("----------------------------------------")
	runMySQLExample()

	// Example 2: PostgreSQL with backward compatibility rules
	fmt.Println("Example 2: PostgreSQL Backward Compatibility")
	fmt.Println("--------------------------------------------")
	runPostgreSQLExample()
}

func runMySQLExample() {
	// Create a config object programmatically for MySQL
	cfg := &config.Config{
		ID: "mysql-backward-compat",
		Rules: []*types.SQLReviewRule{
			// Core backward compatibility rule
			{
				Type:   string(advisor.SchemaRuleSchemaBackwardCompatibility),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_MYSQL,
			},
			// Prevent breaking changes to columns
			{
				Type:   string(advisor.SchemaRuleColumnDisallowChangeType),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_MYSQL,
			},
			{
				Type:   string(advisor.SchemaRuleColumnDisallowDrop),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_MYSQL,
			},
			// Naming conventions with backward compatible payload format
			{
				Type:   string(advisor.SchemaRuleTableNaming),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_MYSQL,
				Payload: map[string]interface{}{
					"format":    "^[a-z]+(_[a-z]+)*$", // snake_case pattern
					"maxLength": 64,                   // MySQL identifier limit
				},
			},
			{
				Type:   string(advisor.SchemaRuleColumnNaming),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_MYSQL,
				Payload: map[string]interface{}{
					"format":    "^[a-z]+(_[a-z]+)*$",
					"maxLength": 64,
				},
			},
			// Number-based payload (old format - still supported)
			{
				Type:   string(advisor.SchemaRuleColumnMaximumCharacterLength),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_MYSQL,
				Payload: map[string]interface{}{
					"number": 20,
				},
			},
			// Array-based payload (disallow certain types)
			{
				Type:   string(advisor.SchemaRuleColumnTypeDisallowList),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_MYSQL,
				Payload: map[string]interface{}{
					"list": []string{"JSON", "BLOB", "TEXT"},
				},
			},
			// Boolean-based payload (comment requirements)
			{
				Type:   string(advisor.SchemaRuleTableCommentConvention),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_MYSQL,
				Payload: map[string]interface{}{
					"required":               true,
					"requiredClassification": false,
					"maxLength":              64,
				},
			},
			// Basic rules without payload
			{
				Type:   string(advisor.SchemaRuleTableRequirePK),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_MYSQL,
			},
		},
	}

	// Create reviewer with config object
	r := reviewer.New(types.Engine_MYSQL).WithConfigObject(cfg)

	// SQL with potential backward compatibility issues
	sql := `
-- Good: Adding nullable column (backward compatible)
ALTER TABLE users ADD COLUMN email VARCHAR(255);

-- Bad: Dropping column (breaks compatibility)
ALTER TABLE users DROP COLUMN phone;

-- Bad: Changing column type (breaks compatibility)
ALTER TABLE users MODIFY COLUMN age BIGINT;

-- Good: Adding index (backward compatible)
CREATE INDEX idx_users_email ON users(email);
`

	// Review the SQL
	ctx := context.Background()
	result, err := r.Review(ctx, sql)
	if err != nil {
		log.Fatalf("Review failed: %v", err)
	}

	// Display results
	displayResults(result)
}

func runPostgreSQLExample() {
	// Create a config object programmatically for PostgreSQL
	cfg := &config.Config{
		ID: "postgres-backward-compat",
		Rules: []*types.SQLReviewRule{
			// Core backward compatibility rule
			{
				Type:   string(advisor.SchemaRuleSchemaBackwardCompatibility),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_POSTGRES,
			},
			// PostgreSQL-specific backward compatibility rules
			{
				Type:   string(advisor.SchemaRuleColumnDisallowChangeType),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_POSTGRES,
			},
			{
				Type:    string(advisor.SchemaRuleStatementDisallowAddColumnWithDefault),
				Level:   types.SQLReviewRuleLevel_WARNING,
				Engine:  types.Engine_POSTGRES,
				Comment: "Adding columns with DEFAULT can lock tables in older PostgreSQL versions",
			},
			{
				Type:    string(advisor.SchemaRuleStatementDisallowAddNotNull),
				Level:   types.SQLReviewRuleLevel_WARNING,
				Engine:  types.Engine_POSTGRES,
				Comment: "Adding NOT NULL requires table scan and can lock tables",
			},
			{
				Type:    string(advisor.SchemaRuleStatementAddFKNotValid),
				Level:   types.SQLReviewRuleLevel_WARNING,
				Engine:  types.Engine_POSTGRES,
				Comment: "Use NOT VALID for foreign keys to avoid long locks",
			},
			// Naming conventions with PostgreSQL identifier limits
			{
				Type:   string(advisor.SchemaRuleTableNaming),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_POSTGRES,
				Payload: map[string]interface{}{
					"format":    "^[a-z]+(_[a-z]+)*$",
					"maxLength": 63, // PostgreSQL identifier limit
				},
			},
			{
				Type:   string(advisor.SchemaRuleColumnNaming),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_POSTGRES,
				Payload: map[string]interface{}{
					"format":    "^[a-z]+(_[a-z]+)*$",
					"maxLength": 63,
				},
			},
			// Primary key type allowlist (backward compatible array format)
			{
				Type:   string(advisor.SchemaRuleIndexPrimaryKeyTypeAllowlist),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_POSTGRES,
				Payload: map[string]interface{}{
					"list": []string{
						"serial", "bigserial",
						"int", "integer", "bigint",
						"int4", "int8",
						"uuid",
					},
				},
			},
			// Numeric limits
			{
				Type:   string(advisor.SchemaRuleStatementMaximumLimitValue),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_POSTGRES,
				Payload: map[string]interface{}{
					"number": 1000,
				},
			},
			{
				Type:   string(advisor.SchemaRuleColumnMaximumCharacterLength),
				Level:  types.SQLReviewRuleLevel_WARNING,
				Engine: types.Engine_POSTGRES,
				Payload: map[string]interface{}{
					"number": 20,
				},
			},
			// Safe index creation
			{
				Type:    string(advisor.SchemaRuleCreateIndexConcurrently),
				Level:   types.SQLReviewRuleLevel_WARNING,
				Engine:  types.Engine_POSTGRES,
				Comment: "Use CONCURRENTLY to avoid blocking writes during index creation",
			},
			// Basic rules
			{
				Type:   string(advisor.SchemaRuleTableRequirePK),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_POSTGRES,
			},
		},
	}

	// Create reviewer with config object
	r := reviewer.New(types.Engine_POSTGRES).WithConfigObject(cfg)

	// SQL with potential backward compatibility issues
	sql := `
-- Good: Adding nullable column without default (backward compatible)
ALTER TABLE users ADD COLUMN email VARCHAR(255);

-- Bad: Adding column with DEFAULT (may lock table)
ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'active';

-- Bad: Adding NOT NULL (requires table scan)
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

-- Good: Creating index concurrently (backward compatible)
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);

-- Bad: Creating index without CONCURRENTLY (blocks writes)
CREATE INDEX idx_users_status ON users(status);

-- Bad: Drop table
DROP TABLE users;
`

	// Review the SQL
	ctx := context.Background()
	result, err := r.Review(ctx, sql)
	if err != nil {
		log.Fatalf("Review failed: %v", err)
	}

	// Display results
	displayResults(result)
}

func displayResults(result *reviewer.ReviewResult) {
	fmt.Printf("Summary: %d total (%d errors, %d warnings)\n\n",
		result.Summary.Total,
		result.Summary.Errors,
		result.Summary.Warnings)

	if result.IsClean() {
		fmt.Println("✅ No issues found - all SQL passes backward compatibility checks!")
		return
	}

	// Group and display findings
	for _, advice := range result.Advices {
		statusSymbol := "⚠️ "
		if advice.Status == types.Advice_ERROR {
			statusSymbol = "❌"
		}

		fmt.Printf("%s %s\n", statusSymbol, advice.Title)
		fmt.Printf("   %s\n", advice.Content)

		if advice.StartPosition != nil && advice.StartPosition.Line > 0 {
			fmt.Printf("   Line: %d", advice.StartPosition.Line)
			if advice.StartPosition.Column > 0 {
				fmt.Printf(", Column: %d", advice.StartPosition.Column)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}

// BACKWARD COMPATIBILITY NOTES:
//
// This example demonstrates various backward compatible payload formats:
//
// 1. STRING TYPE (naming rules):
//    Old format: Just "format" field
//    New format: "format" + "maxLength"
//    Compatibility: Old configs work, new field is optional
//
// 2. NUMBER TYPE (limits):
//    Format: { "number": 123 }
//    Compatibility: Accepts int or float64
//
// 3. ARRAY TYPE (lists):
//    Format: { "list": ["item1", "item2"] }
//    Compatibility: Empty array [] disables check
//
// 4. BOOLEAN TYPE (flags):
//    Format: { "required": true, "maxLength": 64 }
//    Compatibility: Missing booleans default to false
//
// 5. MULTI-COMPONENT (complex):
//    Format: { "required": true, "requiredClassification": false, "maxLength": 64 }
//    Compatibility: Each component has independent defaults
//
// KEY PRINCIPLES:
// - All payload fields are optional and have sensible defaults
// - Missing fields use defaults from config/schema.yaml
// - Type conversions handle int/float64 automatically
// - Empty arrays disable checks (permissive default)
// - New fields can be added without breaking old configs
