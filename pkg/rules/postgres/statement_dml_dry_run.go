package postgres

import (
	"context"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*StatementDMLDryRunAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleStatementDMLDryRun), &StatementDMLDryRunAdvisor{})
}

// StatementDMLDryRunAdvisor is the advisor checking for DML dry run.
// TODO: This rule requires database connection to run EXPLAIN queries.
// Implementation is complex and requires:
// 1. Database driver integration
// 2. Transaction management
// 3. EXPLAIN query execution
// 4. Result parsing and analysis
type StatementDMLDryRunAdvisor struct{}

// Check checks for DML dry run.
func (*StatementDMLDryRunAdvisor) Check(_ context.Context, _ advisor.Context) ([]*types.Advice, error) {
	// TODO: Implement DML dry run checking
	// This requires:
	// - Database connection (checkCtx.Driver)
	// - EXPLAIN query execution for DML statements
	// - Parsing EXPLAIN output
	// - Checking affected rows against limits
	return nil, nil
}
