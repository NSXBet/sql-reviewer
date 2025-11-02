// Package main demonstrates filtering and processing review results.
//
// This example shows how to:
//   - Filter results by status (ERROR, WARNING)
//   - Filter results by error code
//   - Count findings by type
//   - Implement custom result processing
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nsxbet/sql-reviewer/pkg/reviewer"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

func main() {
	r := reviewer.New(types.Engine_MYSQL)

	// Multiple statements with various issues
	sql := `
	-- Missing primary key
	CREATE TABLE logs (
		id INT,
		message TEXT,
		created_at TIMESTAMP
	);

	-- Good table
	CREATE TABLE users (
		id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'User ID',
		username VARCHAR(50) NOT NULL COMMENT 'Username',
		email VARCHAR(100) NOT NULL COMMENT 'Email address'
	) ENGINE=InnoDB COMMENT 'User accounts';

	-- Using SELECT *
	SELECT * FROM users WHERE username = 'admin';

	-- Missing WHERE clause
	DELETE FROM logs;
	`

	ctx := context.Background()
	result, err := r.Review(ctx, sql)
	if err != nil {
		log.Fatalf("Review failed: %v", err)
	}

	// Summary
	fmt.Println("=== Review Summary ===")
	fmt.Println(result.String())
	fmt.Println()

	// Filter and display errors only
	errors := result.FilterByStatus(types.Advice_ERROR)
	if len(errors) > 0 {
		fmt.Printf("=== Errors (%d) ===\n", len(errors))
		for i, advice := range errors {
			fmt.Printf("%d. %s\n", i+1, advice.Title)
			fmt.Printf("   %s\n", advice.Content)
			if advice.StartPosition != nil {
				fmt.Printf("   Line: %d\n", advice.StartPosition.Line)
			}
			fmt.Println()
		}
	}

	// Filter and display warnings only
	warnings := result.FilterByStatus(types.Advice_WARNING)
	if len(warnings) > 0 {
		fmt.Printf("=== Warnings (%d) ===\n", len(warnings))
		for i, advice := range warnings {
			fmt.Printf("%d. %s\n", i+1, advice.Title)
			fmt.Printf("   %s\n", advice.Content)
			if advice.StartPosition != nil {
				fmt.Printf("   Line: %d\n", advice.StartPosition.Line)
			}
			fmt.Println()
		}
	}

	// Group by error code
	fmt.Println("=== Findings by Rule ===")
	codeMap := make(map[int32][]*types.Advice)
	for _, advice := range result.Advices {
		codeMap[advice.Code] = append(codeMap[advice.Code], advice)
	}

	for code, advices := range codeMap {
		fmt.Printf("Code %d: %d finding(s)\n", code, len(advices))
		for _, advice := range advices {
			fmt.Printf("  - %s\n", advice.Title)
		}
	}
	fmt.Println()

	// Exit with error code if errors found (useful for CI/CD)
	if result.HasErrors() {
		fmt.Println("⚠ Review found errors - exiting with code 1")
		// In a real application: os.Exit(1)
	} else if result.HasWarnings() {
		fmt.Println("⚠ Review found warnings - consider fixing them")
	} else {
		fmt.Println("✓ All checks passed!")
	}
}
