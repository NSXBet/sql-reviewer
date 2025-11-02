// Package main demonstrates using SQL reviewer with custom configuration.
//
// This example shows how to:
//   - Load rules from a YAML configuration file
//   - Customize rule severity levels
//   - Enable/disable specific rules
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nsxbet/sql-reviewer/pkg/reviewer"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

func main() {
	// Create reviewer
	r := reviewer.New(types.Engine_MYSQL)

	// Load custom configuration if provided
	configFile := "rules.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	if _, err := os.Stat(configFile); err == nil {
		if err := r.WithConfig(configFile); err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		fmt.Printf("Loaded configuration from %s\n\n", configFile)
	} else {
		fmt.Println("Using default configuration")
		fmt.Println()
	}

	// SQL to review
	sql := `
	CREATE TABLE products (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255) NOT NULL COMMENT 'Product name',
		price DECIMAL(10,2) NOT NULL COMMENT 'Product price',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB COMMENT 'Product catalog';
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
		fmt.Println("✓ SQL passes all quality checks!")
		return
	}

	// Display all findings with details
	for _, advice := range result.Advices {
		statusSymbol := "⚠"
		if advice.Status == types.Advice_ERROR {
			statusSymbol = "✗"
		}

		fmt.Printf("%s [%v] %s\n", statusSymbol, advice.Status, advice.Title)
		fmt.Printf("  %s\n", advice.Content)

		if advice.StartPosition != nil {
			fmt.Printf("  Location: line %d", advice.StartPosition.Line)
			if advice.StartPosition.Column > 0 {
				fmt.Printf(", column %d", advice.StartPosition.Column)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}
