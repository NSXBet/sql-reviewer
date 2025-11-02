package reviewer

import (
	"database/sql"

	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ReviewOption is a functional option for customizing review behavior.
type ReviewOption func(*reviewOptions)

// reviewOptions holds optional configuration for a review operation.
type reviewOptions struct {
	catalog    catalogInterface
	driver     *sql.DB
	changeType types.PlanCheckRunConfig_ChangeDatabaseType
}

// catalogInterface is the interface required for catalog implementations.
// It matches the advisor package's internal catalogInterface.
type catalogInterface interface {
	GetFinder() *catalog.Finder
}

// WithCatalog provides database schema catalog for rules that need it.
//
// Some rules query the database schema to validate statements
// (e.g., checking if a column exists before dropping it).
//
// The catalog parameter must implement GetFinder() *catalog.Finder.
//
// Example:
//
//	catalog := // ... your catalog implementation
//	result, err := r.Review(ctx, sql, WithCatalog(catalog))
func WithCatalog(cat catalogInterface) ReviewOption {
	return func(opts *reviewOptions) {
		opts.catalog = cat
	}
}

// WithDriver provides a database connection for rules that execute queries.
//
// Some rules need to execute queries against the database
// (e.g., dry-run checks, EXPLAIN analysis).
//
// Example:
//
//	db, _ := sql.Open("mysql", dsn)
//	result, err := r.Review(ctx, sql, WithDriver(db))
func WithDriver(driver *sql.DB) ReviewOption {
	return func(opts *reviewOptions) {
		opts.driver = driver
	}
}

// WithChangeType specifies the type of database change being reviewed.
//
// This affects which rules are applied and how they behave.
// Common types include DDL, DML, and DDL_DML.
//
// Example:
//
//	result, err := r.Review(ctx, sql,
//	    WithChangeType(types.PlanCheckRunConfig_DDL))
func WithChangeType(changeType types.PlanCheckRunConfig_ChangeDatabaseType) ReviewOption {
	return func(opts *reviewOptions) {
		opts.changeType = changeType
	}
}
