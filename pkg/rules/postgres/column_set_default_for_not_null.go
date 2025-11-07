package postgres

import (
	"context"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*ColumnSetDefaultForNotNullAdvisor)(nil)

func init() {
	advisor.Register(
		types.Engine_POSTGRES,
		advisor.Type(advisor.SchemaRuleColumnSetDefaultForNotNull),
		&ColumnSetDefaultForNotNullAdvisor{},
	)
}

type ColumnSetDefaultForNotNullAdvisor struct{}

func (*ColumnSetDefaultForNotNullAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	// TODO: Implement this rule for PostgreSQL
	// Require NOT NULL columns to have a DEFAULT value
	return nil, nil
}
