// Package main demonstrates basic usage of the SQL reviewer as a library.
//
// This example shows how to:
//   - Create a reviewer with default configuration
//   - Review SQL statements
//   - Process and display results
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nsxbet/sql-reviewer/pkg/reviewer"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

func main() {
	// Create a reviewer for MySQL with default rules
	r := reviewer.New(types.Engine_MYSQL)

	// SQL to review
	sql := `
	CREATE TABLE users (
		id INT,
		name VARCHAR(100),
		email VARCHAR(100)
	);
	`

	// Review the SQL
	ctx := context.Background()
	result, err := r.Review(ctx, sql)
	if err != nil {
		log.Fatalf("Review failed: %v", err)
	}

	// Display results
	fmt.Println(result.String())
	fmt.Println()

	if result.IsClean() {
		fmt.Println("✓ No issues found!")
		return
	}

	// Show errors
	if result.HasErrors() {
		fmt.Println("Errors:")
		for _, advice := range result.FilterByStatus(types.Advice_ERROR) {
			fmt.Printf("  - %s: %s\n", advice.Title, advice.Content)
			if advice.StartPosition != nil {
				fmt.Printf("    at line %d\n", advice.StartPosition.Line)
			}
		}
		fmt.Println()
	}

	// Show warnings
	if result.HasWarnings() {
		fmt.Println("Warnings:")
		for _, advice := range result.FilterByStatus(types.Advice_WARNING) {
			fmt.Printf("  - %s: %s\n", advice.Title, advice.Content)
			if advice.StartPosition != nil {
				fmt.Printf("    at line %d\n", advice.StartPosition.Line)
			}
		}
	}
}
