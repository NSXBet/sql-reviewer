package postgres

import (
	"context"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementNonTransactionalAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleStatementNonTransactional),
		&StatementNonTransactionalAdvisor{},
	)
}

// StatementNonTransactionalAdvisor is the advisor checking for non-transactional statements.
// TODO: This rule checks for statements that cannot be run within a transaction.
// Implementation requires:
// 1. Identifying non-transactional DDL statements
// 2. Checking transaction context
// 3. Detecting mixed transactional/non-transactional statements
type StatementNonTransactionalAdvisor struct{}

// Check checks for non-transactional statements.
func (*StatementNonTransactionalAdvisor) Check(_ context.Context, _ advisor.Context) ([]*types.Advice, error) {
	// TODO: Implement non-transactional statement checking
	// This requires:
	// - Parsing statements to identify non-transactional operations
	// - Examples: CREATE INDEX CONCURRENTLY, VACUUM, etc.
	// - Checking if statements are within a transaction block
	return nil, nil
}
