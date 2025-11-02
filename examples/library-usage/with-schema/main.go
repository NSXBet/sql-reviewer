// Package main demonstrates using SQL reviewer with database schema context.
//
// This example shows how to:
//   - Provide existing database schema metadata
//   - Validate ALTER statements against existing schema
//   - Check for breaking changes
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nsxbet/sql-reviewer/pkg/reviewer"
	"github.com/nsxbet/sql-reviewer/pkg/types"

	_ "github.com/nsxbet/sql-reviewer/pkg/rules/mysql"
)

func main() {
	r := reviewer.New(types.Engine_MYSQL)

	// Create existing schema metadata
	// In a real application, this would come from your database
	schema := &types.DatabaseSchemaMetadata{
		Name: "mydb",
		Schemas: []*types.SchemaMetadata{
			{
				Name: "public",
				Tables: []*types.TableMetadata{
					{
						Name: "users",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Type:     "INT",
								Nullable: false,
							},
							{
								Name:     "username",
								Type:     "VARCHAR(50)",
								Nullable: false,
							},
							{
								Name:     "email",
								Type:     "VARCHAR(100)",
								Nullable: false,
							},
							{
								Name:     "created_at",
								Type:     "TIMESTAMP",
								Nullable: true,
							},
						},
						Indexes: []*types.IndexMetadata{
							{
								Name:    "PRIMARY",
								Primary: true,
								Unique:  true,
								Expressions: []string{"id"},
							},
							{
								Name:        "idx_username",
								Primary:     false,
								Unique:      true,
								Expressions: []string{"username"},
							},
						},
					},
				},
			},
		},
	}

	// SQL that modifies existing schema
	sql := `
	-- Add a new column
	ALTER TABLE users ADD COLUMN phone VARCHAR(20) COMMENT 'Phone number';

	-- Drop a column (potentially breaking)
	ALTER TABLE users DROP COLUMN email;

	-- Modify column type (potentially breaking)
	ALTER TABLE users MODIFY COLUMN username VARCHAR(100);
	`

	fmt.Println("=== Reviewing schema changes ===")
	fmt.Println()

	// Review with schema context
	ctx := context.Background()
	result, err := r.ReviewWithSchema(ctx, sql, schema)
	if err != nil {
		log.Fatalf("Review failed: %v", err)
	}

	// Display results
	fmt.Println(result.String())
	fmt.Println()

	if result.IsClean() {
		fmt.Println("✓ Schema changes are safe!")
		return
	}

	// Show all findings
	for _, advice := range result.Advices {
		statusSymbol := "ℹ"
		switch advice.Status {
		case types.Advice_ERROR:
			statusSymbol = "✗"
		case types.Advice_WARNING:
			statusSymbol = "⚠"
		case types.Advice_SUCCESS:
			statusSymbol = "✓"
		}

		fmt.Printf("%s [%v] %s\n", statusSymbol, advice.Status, advice.Title)
		fmt.Printf("  %s\n", advice.Content)

		if advice.StartPosition != nil {
			fmt.Printf("  Location: line %d\n", advice.StartPosition.Line)
		}
		fmt.Println()
	}

	// Analyze impact
	fmt.Println("=== Impact Analysis ===")
	if result.HasErrors() {
		fmt.Println("⚠ Critical: This migration contains breaking changes!")
		fmt.Println("  Review carefully before applying to production.")
	} else if result.HasWarnings() {
		fmt.Println("⚠ Caution: This migration has potential issues.")
		fmt.Println("  Consider the warnings before proceeding.")
	}
}
