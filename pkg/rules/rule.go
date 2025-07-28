package rules

import (
	"context"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// Rule defines the interface for SQL review rules
type Rule interface {
	// Check runs the rule against SQL statements and returns advice
	Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error)
	
	// GetType returns the rule type this implementation handles
	GetType() string
}

// Registry holds all registered rules
type Registry struct {
	rules map[string]Rule
}

// NewRegistry creates a new rule registry
func NewRegistry() *Registry {
	return &Registry{
		rules: make(map[string]Rule),
	}
}

// Register registers a rule implementation
func (r *Registry) Register(rule Rule) {
	r.rules[rule.GetType()] = rule
}

// Get retrieves a rule by type
func (r *Registry) Get(ruleType string) (Rule, bool) {
	rule, exists := r.rules[ruleType]
	return rule, exists
}

// GetAll returns all registered rules
func (r *Registry) GetAll() map[string]Rule {
	return r.rules
}

// Default global registry
var DefaultRegistry = NewRegistry()