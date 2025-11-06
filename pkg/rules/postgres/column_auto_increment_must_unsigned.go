package postgres

import (
	"context"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnAutoIncrementMustUnsignedAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleColumnAutoIncrementMustUnsigned), &ColumnAutoIncrementMustUnsignedAdvisor{})
}

type ColumnAutoIncrementMustUnsignedAdvisor struct{}

func (*ColumnAutoIncrementMustUnsignedAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	// TODO: Implement this rule for PostgreSQL
	// PostgreSQL does not have unsigned types
	// This rule is MySQL-specific and may not be applicable to PostgreSQL
	return nil, nil
}
