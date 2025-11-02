// Package pkg provides comprehensive SQL review and validation functionality for Go applications.
//
// SQL Reviewer offers both high-level and low-level APIs for reviewing SQL statements
// against configurable quality and style rules, supporting multiple database engines.
//
// # Package Structure
//
// The pkg directory contains several specialized packages:
//
//   - reviewer: High-level API for easy SQL review (recommended starting point)
//   - advisor: Low-level rule execution engine and registration system
//   - types: Core type definitions and data structures
//   - config: Configuration loading and management
//   - catalog: Database schema metadata and catalog functionality
//   - rules: Engine-specific rule implementations (mysql, postgres, etc.)
//   - mysqlparser: ANTLR-based MySQL SQL parser
//   - logger: Logging abstraction layer
//
// # Getting Started
//
// For most use cases, start with the reviewer package:
//
//	import (
//	    "github.com/nsxbet/sql-reviewer/pkg/reviewer"
//	    "github.com/nsxbet/sql-reviewer/pkg/types"
//	    _ "github.com/nsxbet/sql-reviewer/pkg/rules/mysql"
//	)
//
//	func main() {
//	    r := reviewer.New(types.Engine_MYSQL)
//	    result, err := r.Review(context.Background(), sqlStatements)
//	    // Process results...
//	}
//
// # Rule Categories
//
// SQL Reviewer includes comprehensive rules organized by category:
//
// Engine Rules: Database engine configuration (InnoDB, storage engines)
//
// Naming Rules: Enforce consistent naming conventions
//   - Table names (format, length limits)
//   - Column names (format, length limits)
//   - Index names (PK, UK, FK, IDX prefixes)
//   - Auto-increment column naming
//   - Keyword avoidance
//
// Table Rules: Table-level constraints and best practices
//   - Primary key requirements
//   - Foreign key policies
//   - Comment requirements
//   - Partition restrictions
//   - Trigger policies
//   - Duplicate index detection
//   - Text field limits
//   - Charset/collation requirements
//
// Column Rules: Column definitions and constraints
//   - Required columns
//   - NULL restrictions
//   - Type change policies
//   - Default value requirements
//   - Auto-increment rules
//   - Type restrictions
//   - Character length limits
//   - Charset/collation settings
//
// Index Rules: Index design and optimization
//   - Duplicate column prevention
//   - Key number limits
//   - Type restrictions
//   - Total number limits
//   - Redundancy detection
//
// Statement Rules: SQL statement quality and safety
//   - SELECT * prohibition
//   - WHERE clause requirements
//   - JOIN optimization
//   - LIMIT/ORDER BY policies
//   - Transaction controls
//   - Execution plan analysis
//   - Affected row limits
//
// Schema Rules: Database schema-level validation
//   - Backward compatibility checks
//   - Breaking change detection
//   - Drop restrictions
//
// System Rules: System-level configurations
//   - Charset restrictions
//   - Collation restrictions
//   - Procedure/function policies
//   - Event policies
//   - View policies
//
// # Configuration
//
// Rules can be configured via YAML/JSON files or programmatically:
//
//	r := reviewer.New(types.Engine_MYSQL)
//	if err := r.WithConfig("custom-rules.yaml"); err != nil {
//	    log.Fatal(err)
//	}
//
// # Advanced Features
//
// Schema-aware validation:
//
//	schema := &types.DatabaseSchemaMetadata{...}
//	result, err := r.ReviewWithSchema(ctx, sql, schema)
//
// Database connection for EXPLAIN analysis:
//
//	db, _ := sql.Open("mysql", dsn)
//	result, err := r.Review(ctx, sql, reviewer.WithDriver(db))
//
// Result filtering:
//
//	errors := result.FilterByStatus(types.Advice_ERROR)
//	warnings := result.FilterByStatus(types.Advice_WARNING)
//
// # Custom Rules
//
// Implement custom validation rules by satisfying the Advisor interface:
//
//	type MyRule struct{}
//
//	func (r *MyRule) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
//	    // Validation logic
//	    return advices, nil
//	}
//
//	func init() {
//	    advisor.Register(types.Engine_MYSQL, "custom.my-rule", &MyRule{})
//	}
//
// # Thread Safety
//
// All public APIs are safe for concurrent use by multiple goroutines.
// Reviewer instances can be reused across multiple review operations.
//
// # Error Handling
//
// Review operations distinguish between:
//   - Validation findings (returned as Advice in ReviewResult)
//   - System errors (returned as error from Review/ReviewWithSchema)
//
// Individual rule failures are logged but don't cause Review to return an error,
// allowing partial results even when some rules fail.
//
// # Performance
//
// Review operations support context cancellation for timeout control:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	result, err := r.Review(ctx, sql)
//
// Large SQL files are processed efficiently with statement-by-statement parsing
// and rule evaluation. Context cancellation is checked between rules.
//
// # Documentation
//
// Complete documentation and examples:
//   - Package documentation: https://pkg.go.dev/github.com/nsxbet/sql-reviewer/pkg
//   - Library guide: pkg/README.md
//   - Examples: examples/library-usage/
//   - Main README: README.md
//
// # License
//
// See LICENSE file for details.
package pkg
