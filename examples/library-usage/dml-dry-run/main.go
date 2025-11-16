// Package main demonstrates DML dry-run validation with graceful degradation.
//
// This example shows how to:
//   - Enable DML dry-run validation for PostgreSQL and MySQL
//   - Use WithDriver option to provide database connection
//   - Demonstrate graceful skip when no connection is available
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/config"
	"github.com/nsxbet/sql-reviewer/pkg/reviewer"
	_ "github.com/nsxbet/sql-reviewer/pkg/rules/mysql"
	_ "github.com/nsxbet/sql-reviewer/pkg/rules/postgres"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

func main() {
	fmt.Println("=== SQL Reviewer: DML Dry-Run Validation Example ===\n")

	// Example 1: PostgreSQL with graceful skip (no database connection)
	fmt.Println("Example 1: PostgreSQL DML Dry-Run (Graceful Skip - No Database)")
	fmt.Println("------------------------------------------------------------------")
	postgresGracefulSkip()

	fmt.Println("\n")

	// Example 2: PostgreSQL with database connection (if available)
	fmt.Println("Example 2: PostgreSQL DML Dry-Run (With Database Connection)")
	fmt.Println("-------------------------------------------------------------")
	postgresWithDatabase()

	fmt.Println("\n")

	// Example 3: MySQL with graceful skip
	fmt.Println("Example 3: MySQL DML Dry-Run (Graceful Skip - No Database)")
	fmt.Println("-----------------------------------------------------------")
	mysqlGracefulSkip()

	fmt.Println("\n")

	// Example 4: MySQL with database connection (if available)
	fmt.Println("Example 4: MySQL DML Dry-Run (With Database Connection)")
	fmt.Println("--------------------------------------------------------")
	mysqlWithDatabase()
}

// postgresGracefulSkip demonstrates graceful skip when no database connection
func postgresGracefulSkip() {
	// Create reviewer for PostgreSQL
	r := createPostgreSQLReviewer()

	// SQL statements
	sql := `
	INSERT INTO users (id, name, email) VALUES (1, 'John', 'john@example.com');
	UPDATE users SET name = 'Jane' WHERE id = 1;
	DELETE FROM users WHERE id = 2;
	`

	// Review WITHOUT database connection
	ctx := context.Background()
	result, err := r.Review(ctx, sql)
	if err != nil {
		log.Printf("Review failed: %v", err)
		return
	}

	fmt.Println("✓ DML dry-run rule gracefully skipped (no database connection)")
	fmt.Println("✓ No errors reported - validation requires database connection")

	if result.IsClean() {
		fmt.Println("✓ Clean result (as expected)")
	} else {
		displayResults(result)
	}
}

// postgresWithDatabase demonstrates PostgreSQL DML dry-run with database connection
func postgresWithDatabase() {
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		fmt.Println("⚠️  POSTGRES_URL not set. Skipping database connection example.")
		fmt.Println("   Set POSTGRES_URL to test: export POSTGRES_URL='postgres://user:pass@localhost/dbname'")
		return
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", pgURL)
	if err != nil {
		log.Printf("Failed to connect to PostgreSQL: %v", err)
		return
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		log.Printf("Failed to ping PostgreSQL: %v", err)
		return
	}

	fmt.Println("✓ Connected to PostgreSQL")

	// Create reviewer
	r := createPostgreSQLReviewer()

	// SQL with intentional error (non-existent table)
	sql := `
	INSERT INTO nonexistent_table (id, name) VALUES (1, 'test');
	UPDATE users SET email = 'test@example.com' WHERE id = 1;
	DELETE FROM products WHERE id = 999;
	`

	// Review WITH database connection
	ctx := context.Background()
	result, err := r.Review(ctx, sql, reviewer.WithDriver(db))
	if err != nil {
		log.Printf("Review failed: %v", err)
		return
	}

	// Display results
	displayResults(result)
}

// mysqlGracefulSkip demonstrates graceful skip when no database connection
func mysqlGracefulSkip() {
	// Create reviewer for MySQL
	r := createMySQLReviewer()

	// SQL statements
	sql := `
	INSERT INTO users (id, name, email) VALUES (1, 'John', 'john@example.com');
	UPDATE users SET name = 'Jane' WHERE id = 1;
	DELETE FROM users WHERE id = 2;
	`

	// Review WITHOUT database connection
	ctx := context.Background()
	result, err := r.Review(ctx, sql)
	if err != nil {
		log.Printf("Review failed: %v", err)
		return
	}

	fmt.Println("✓ DML dry-run rule gracefully skipped (no database connection)")
	fmt.Println("✓ No errors reported - validation requires database connection")

	if result.IsClean() {
		fmt.Println("✓ Clean result (as expected)")
	} else {
		displayResults(result)
	}
}

// mysqlWithDatabase demonstrates MySQL DML dry-run with database connection
func mysqlWithDatabase() {
	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		fmt.Println("⚠️  MYSQL_DSN not set. Skipping database connection example.")
		fmt.Println("   Set MYSQL_DSN to test: export MYSQL_DSN='user:pass@tcp(localhost:3306)/dbname'")
		return
	}

	// Connect to MySQL
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		log.Printf("Failed to connect to MySQL: %v", err)
		return
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		log.Printf("Failed to ping MySQL: %v", err)
		return
	}

	fmt.Println("✓ Connected to MySQL")

	// Create reviewer
	r := createMySQLReviewer()

	// SQL with intentional error (non-existent table)
	sql := `
	INSERT INTO nonexistent_table (id, name) VALUES (1, 'test');
	UPDATE users SET email = 'test@example.com' WHERE id = 1;
	`

	// Review WITH database connection
	ctx := context.Background()
	result, err := r.Review(ctx, sql, reviewer.WithDriver(db))
	if err != nil {
		log.Printf("Review failed: %v", err)
		return
	}

	// Display results
	displayResults(result)
}

// createPostgreSQLReviewer creates a reviewer with DML dry-run rule enabled
func createPostgreSQLReviewer() *reviewer.Reviewer {
	cfg := &config.Config{
		ID: "postgres-dml-dry-run",
		Rules: []*types.SQLReviewRule{
			{
				Type:   string(advisor.SchemaRuleStatementDMLDryRun),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_POSTGRES,
			},
		},
	}

	return reviewer.New(types.Engine_POSTGRES).WithConfigObject(cfg)
}

// createMySQLReviewer creates a reviewer with DML dry-run rule enabled
func createMySQLReviewer() *reviewer.Reviewer {
	cfg := &config.Config{
		ID: "mysql-dml-dry-run",
		Rules: []*types.SQLReviewRule{
			{
				Type:   string(advisor.SchemaRuleStatementDMLDryRun),
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_MYSQL,
			},
		},
	}

	return reviewer.New(types.Engine_MYSQL).WithConfigObject(cfg)
}

// displayResults shows the review results
func displayResults(result *reviewer.ReviewResult) {
	fmt.Printf("Summary: %d total (%d errors, %d warnings)\n\n",
		result.Summary.Total,
		result.Summary.Errors,
		result.Summary.Warnings)

	if result.IsClean() {
		fmt.Println("✓ All DML statements passed dry-run validation!")
		return
	}

	// Show errors
	if result.Summary.Errors > 0 {
		fmt.Println("❌ Errors:")
		for _, advice := range result.Advices {
			if advice.Status == types.Advice_ERROR {
				fmt.Printf("  • %s\n", advice.Content)
				if advice.StartPosition != nil {
					fmt.Printf("    at line %d\n", advice.StartPosition.Line+1)
				}
			}
		}
		fmt.Println()
	}

	// Show warnings
	if result.Summary.Warnings > 0 {
		fmt.Println("⚠️  Warnings:")
		for _, advice := range result.Advices {
			if advice.Status == types.Advice_WARNING {
				fmt.Printf("  • %s\n", advice.Content)
				if advice.StartPosition != nil {
					fmt.Printf("    at line %d\n", advice.StartPosition.Line+1)
				}
			}
		}
	}
}
