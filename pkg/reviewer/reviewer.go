// Package reviewer provides a high-level API for SQL statement review and validation.
//
// This package offers a simplified interface for reviewing SQL against configurable rules,
// making it easy to integrate SQL quality checks into Go applications.
//
// # Quick Start
//
//	// Create a reviewer for MySQL
//	r := reviewer.New(types.Engine_MYSQL)
//
//	// Review SQL statements
//	result, err := r.Review(context.Background(), "CREATE TABLE users (id INT);")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check results
//	fmt.Printf("Found %d issues\n", result.Summary.Total)
//	for _, advice := range result.Advices {
//	    fmt.Printf("[%s] %s\n", advice.Status, advice.Content)
//	}
//
// # Using Custom Configuration
//
//	r := reviewer.New(types.Engine_MYSQL)
//	if err := r.WithConfig("custom-rules.yaml"); err != nil {
//	    log.Fatal(err)
//	}
//	result, err := r.Review(ctx, sqlStatements)
//
// # With Database Schema Context
//
//	schema := &types.DatabaseSchemaMetadata{
//	    Name: "mydb",
//	    Schemas: []*types.SchemaMetadata{...},
//	}
//	result, err := r.ReviewWithSchema(ctx, sqlStatements, schema)
package reviewer

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/config"
	"github.com/nsxbet/sql-reviewer/pkg/logger"
	_ "github.com/nsxbet/sql-reviewer/pkg/rules/mysql"
	_ "github.com/nsxbet/sql-reviewer/pkg/rules/postgres"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

// Reviewer provides a high-level API for SQL review operations.
// It encapsulates configuration management and rule execution.
//
// Reviewer is safe for concurrent use by multiple goroutines.
type Reviewer struct {
	config *config.Config
	engine types.Engine
}

// New creates a new Reviewer for the specified database engine with default configuration.
//
// The default configuration attempts to load rules from config/schema.yaml or schema.yaml.
// If the schema file is not found, an empty configuration is used (no rules enabled).
//
// Use WithConfig or WithConfigObject to customize the rules.
//
// Example:
//
//	r := reviewer.New(types.Engine_MYSQL)
//	result, err := r.Review(ctx, "CREATE TABLE users (id INT);")
func New(engine types.Engine) *Reviewer {
	return &Reviewer{
		config: loadDefaultConfig(engine),
		engine: engine,
	}
}

// loadDefaultConfig attempts to load the default configuration from schema.yaml
func loadDefaultConfig(engine types.Engine) *config.Config {
	// Try to find schema.yaml by searching upwards from current directory
	schemaPath := findSchemaFile()
	if schemaPath == "" {
		// Schema file not found, use empty config
		return config.DefaultConfig("default")
	}

	// Load schema rules
	schemaRules, err := config.LoadSchema(schemaPath)
	if err != nil {
		// Failed to load schema, use empty config
		return config.DefaultConfig("default")
	}

	// Convert schema rules to configuration with default payloads
	cfg, err := config.ConvertSchemaRulesToConfig(schemaRules, engine)
	if err != nil {
		// Failed to convert schema rules, use empty config
		return config.DefaultConfig("default")
	}

	return cfg
}

// findSchemaFile searches for schema.yaml starting from current directory and moving upwards
func findSchemaFile() string {
	// List of possible schema file locations (in order of preference)
	candidates := []string{
		"config/schema.yaml",
		"schema.yaml",
		"../../config/schema.yaml", // For when running tests in pkg/reviewer/
		"../config/schema.yaml",    // For when running tests in pkg/
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// WithConfig loads rule configuration from a YAML or JSON file.
// This replaces the current configuration.
//
// Returns an error if the file cannot be read or parsed.
//
// Example:
//
//	r := reviewer.New(types.Engine_MYSQL)
//	if err := r.WithConfig("custom-rules.yaml"); err != nil {
//	    return err
//	}
func (r *Reviewer) WithConfig(filename string) error {
	cfg, err := config.LoadFromFile(filename)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", filename, err)
	}
	r.config = cfg
	return nil
}

// WithConfigObject sets a custom configuration object directly.
// This replaces the current configuration.
//
// Returns the Reviewer for method chaining.
//
// Example:
//
//	cfg := &config.Config{...}
//	r := reviewer.New(types.Engine_MYSQL).WithConfigObject(cfg)
func (r *Reviewer) WithConfigObject(cfg *config.Config) *Reviewer {
	r.config = cfg
	return r
}

// Review runs all enabled rules against the provided SQL statements.
// It returns a ReviewResult containing all findings and a summary.
//
// The context parameter supports cancellation and timeouts. Rule execution will
// stop when the context is cancelled, returning partial results.
//
// Optional ReviewOption parameters can customize the review behavior:
//
//	result, err := r.Review(ctx, sql,
//	    WithCatalog(catalog),
//	    WithChangeType(types.PlanCheckRunConfig_DDL),
//	)
//
// Returns an error only if the review process itself fails. Individual rule
// failures are logged but do not cause Review to return an error.
func (r *Reviewer) Review(ctx context.Context, sql string, opts ...ReviewOption) (*ReviewResult, error) {
	return r.ReviewWithSchema(ctx, sql, nil, opts...)
}

// ReviewWithSchema runs all enabled rules with database schema context.
// This is useful for rules that need to validate against existing schema
// (e.g., checking if a column exists before altering it).
//
// The schema parameter provides metadata about the database structure.
// Pass nil if schema context is not needed.
//
// Example:
//
//	schema := &types.DatabaseSchemaMetadata{
//	    Name: "mydb",
//	    Schemas: []*types.SchemaMetadata{
//	        {
//	            Name: "public",
//	            Tables: []*types.TableMetadata{...},
//	        },
//	    },
//	}
//	result, err := r.ReviewWithSchema(ctx, sql, schema)
func (r *Reviewer) ReviewWithSchema(
	ctx context.Context,
	sql string,
	schema *types.DatabaseSchemaMetadata,
	opts ...ReviewOption,
) (*ReviewResult, error) {
	// Get rules for the configured engine
	rules := r.config.GetRulesForEngine(r.engine)

	// Build review options
	reviewOpts := &reviewOptions{}
	for _, opt := range opts {
		opt(reviewOpts)
	}

	// Set query logging level if requested
	if reviewOpts.queryLogging {
		customLogger := logger.NewWithLevel(slog.LevelDebug)
		slog.SetDefault(customLogger.GetSlogLogger())
	}

	// Prepare check context
	checkCtx := advisor.Context{
		DBType:     r.engine,
		DBSchema:   schema,
		Statements: sql,
		Driver:     reviewOpts.driver,
		ChangeType: reviewOpts.changeType,
	}

	// Set catalog if provided (using type assertion to satisfy internal interface)
	if reviewOpts.catalog != nil {
		checkCtx.Catalog = reviewOpts.catalog
	}

	// Collect all advices
	var allAdvices []*types.Advice

	// Process each rule
	for _, rule := range rules {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return &ReviewResult{
				Advices: allAdvices,
				Summary: calculateSummary(allAdvices),
			}, ctx.Err()
		default:
		}

		// Skip rules for different engines
		if rule.Engine != types.Engine_ENGINE_UNSPECIFIED && rule.Engine != r.engine {
			continue
		}

		// Skip disabled rules
		if rule.Level == types.SQLReviewRuleLevel_DISABLED {
			continue
		}

		// Set up the context for this rule check
		ruleCheckContext := checkCtx
		ruleCheckContext.Rule = rule

		// Run the check
		advices, err := advisor.Check(ctx, r.engine, advisor.Type(rule.Type), ruleCheckContext)
		if err != nil {
			// Log but don't fail - rule implementations may be incomplete
			// Users can check logs for details
			continue
		}

		allAdvices = append(allAdvices, advices...)
	}

	return &ReviewResult{
		Advices: allAdvices,
		Summary: calculateSummary(allAdvices),
	}, nil
}

// calculateSummary computes aggregate statistics from advices
func calculateSummary(advices []*types.Advice) Summary {
	summary := Summary{}
	for _, advice := range advices {
		summary.Total++
		switch advice.Status {
		case types.Advice_ERROR:
			summary.Errors++
		case types.Advice_WARNING:
			summary.Warnings++
		case types.Advice_SUCCESS:
			summary.Success++
		}
	}
	return summary
}
