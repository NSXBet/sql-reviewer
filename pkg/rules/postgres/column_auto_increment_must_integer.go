package postgres

import (
	"context"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnAutoIncrementMustIntegerAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleColumnAutoIncrementMustInteger), &ColumnAutoIncrementMustIntegerAdvisor{})
}

type ColumnAutoIncrementMustIntegerAdvisor struct{}

func (*ColumnAutoIncrementMustIntegerAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	// TODO: Implement this rule for PostgreSQL
	// PostgreSQL uses SERIAL types which are implicitly integer-based
	// This rule may not be applicable or needs different logic than MySQL
	return nil, nil
}
