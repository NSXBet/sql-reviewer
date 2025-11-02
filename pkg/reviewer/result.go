package reviewer

import (
	"fmt"

	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// ReviewResult contains the results of a SQL review operation.
//
// It includes all findings (advices) from the enabled rules and
// aggregate statistics for quick analysis.
type ReviewResult struct {
	// Advices contains all findings from the review.
	// Empty if no issues were found.
	Advices []*types.Advice

	// Summary provides aggregate statistics about the findings.
	Summary Summary
}

// Summary provides aggregate statistics about review findings.
//
// It categorizes findings by severity level for quick analysis.
type Summary struct {
	// Total number of findings (errors + warnings + success)
	Total int

	// Errors is the count of ERROR-level findings.
	// These typically indicate serious issues that should be fixed.
	Errors int

	// Warnings is the count of WARNING-level findings.
	// These are suggestions for improvement but not critical.
	Warnings int

	// Success is the count of SUCCESS-level findings.
	// These are informational or confirmations.
	Success int
}

// HasErrors returns true if the review found any ERROR-level findings.
//
// This is useful for CI/CD pipelines that should fail on errors:
//
//	if result.HasErrors() {
//	    os.Exit(1)
//	}
func (r *ReviewResult) HasErrors() bool {
	return r.Summary.Errors > 0
}

// HasWarnings returns true if the review found any WARNING-level findings.
func (r *ReviewResult) HasWarnings() bool {
	return r.Summary.Warnings > 0
}

// IsClean returns true if the review found no errors or warnings.
//
// This indicates the SQL passes all enabled quality checks.
func (r *ReviewResult) IsClean() bool {
	return r.Summary.Errors == 0 && r.Summary.Warnings == 0
}

// String returns a human-readable summary of the review results.
//
// Example output:
//
//	Review Results: 5 total (2 errors, 3 warnings, 0 success)
func (r *ReviewResult) String() string {
	return fmt.Sprintf(
		"Review Results: %d total (%d errors, %d warnings, %d success)",
		r.Summary.Total,
		r.Summary.Errors,
		r.Summary.Warnings,
		r.Summary.Success,
	)
}

// FilterByStatus returns a new slice containing only advices with the specified status.
//
// This is useful for processing errors and warnings separately:
//
//	errors := result.FilterByStatus(types.Advice_ERROR)
//	for _, err := range errors {
//	    fmt.Printf("ERROR: %s\n", err.Content)
//	}
func (r *ReviewResult) FilterByStatus(status types.Advice_Status) []*types.Advice {
	filtered := make([]*types.Advice, 0)
	for _, advice := range r.Advices {
		if advice.Status == status {
			filtered = append(filtered, advice)
		}
	}
	return filtered
}

// FilterByCode returns a new slice containing only advices with the specified error code.
//
// This is useful for checking specific rule violations:
//
//	syntaxErrors := result.FilterByCode(int32(types.StatementSyntaxError))
func (r *ReviewResult) FilterByCode(code int32) []*types.Advice {
	filtered := make([]*types.Advice, 0)
	for _, advice := range r.Advices {
		if advice.Code == code {
			filtered = append(filtered, advice)
		}
	}
	return filtered
}
