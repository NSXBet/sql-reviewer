package reviewer

import (
	"database/sql"
	"testing"

	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// mockCatalog implements catalogInterface for testing
type mockCatalog struct {
	finder *catalog.Finder
}

func (m *mockCatalog) GetFinder() *catalog.Finder {
	return m.finder
}

func TestWithCatalog(t *testing.T) {
	mock := &mockCatalog{
		finder: catalog.NewEmptyFinder(&catalog.FinderContext{
			EngineType: types.Engine_MYSQL,
		}),
	}

	opts := &reviewOptions{}
	option := WithCatalog(mock)
	option(opts)

	if opts.catalog == nil {
		t.Error("WithCatalog() did not set catalog")
	}

	if opts.catalog != mock {
		t.Error("WithCatalog() set wrong catalog")
	}
}

func TestWithDriver(t *testing.T) {
	// Note: We're not actually opening a real database connection here
	// In a real test, you might use a mock or test database
	var db *sql.DB // nil is fine for testing the option setting

	opts := &reviewOptions{}
	option := WithDriver(db)
	option(opts)

	if opts.driver != db {
		t.Error("WithDriver() did not set driver correctly")
	}
}

func TestWithChangeType(t *testing.T) {
	tests := []struct {
		name       string
		changeType types.PlanCheckRunConfig_ChangeDatabaseType
	}{
		{
			name:       "DDL",
			changeType: types.PlanCheckRunConfig_DDL,
		},
		{
			name:       "DML",
			changeType: types.PlanCheckRunConfig_DML,
		},
		{
			name:       "SDL",
			changeType: types.PlanCheckRunConfig_SDL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &reviewOptions{}
			option := WithChangeType(tt.changeType)
			option(opts)

			if opts.changeType != tt.changeType {
				t.Errorf("WithChangeType() set %v, want %v", opts.changeType, tt.changeType)
			}
		})
	}
}

func TestReviewOptions_DefaultValues(t *testing.T) {
	opts := &reviewOptions{}

	if opts.catalog != nil {
		t.Error("Default catalog should be nil")
	}

	if opts.driver != nil {
		t.Error("Default driver should be nil")
	}

	// changeType should be zero value
	if opts.changeType != 0 {
		t.Errorf("Default changeType should be 0, got %v", opts.changeType)
	}
}

func TestReviewOptions_MultipleOptions(t *testing.T) {
	mock := &mockCatalog{
		finder: catalog.NewEmptyFinder(&catalog.FinderContext{
			EngineType: types.Engine_MYSQL,
		}),
	}

	opts := &reviewOptions{}

	// Apply multiple options
	options := []ReviewOption{
		WithCatalog(mock),
		WithChangeType(types.PlanCheckRunConfig_DDL),
	}

	for _, opt := range options {
		opt(opts)
	}

	if opts.catalog == nil {
		t.Error("Catalog was not set")
	}

	if opts.changeType != types.PlanCheckRunConfig_DDL {
		t.Error("ChangeType was not set")
	}
}

func TestReviewOptions_Overwrite(t *testing.T) {
	opts := &reviewOptions{}

	// Set initial value
	WithChangeType(types.PlanCheckRunConfig_DDL)(opts)
	if opts.changeType != types.PlanCheckRunConfig_DDL {
		t.Error("Initial changeType not set")
	}

	// Overwrite with new value
	WithChangeType(types.PlanCheckRunConfig_DML)(opts)
	if opts.changeType != types.PlanCheckRunConfig_DML {
		t.Error("ChangeType was not overwritten")
	}
}

func TestCatalogInterface(t *testing.T) {
	// Verify that mockCatalog implements catalogInterface
	var _ catalogInterface = (*mockCatalog)(nil)

	mock := &mockCatalog{
		finder: catalog.NewEmptyFinder(&catalog.FinderContext{
			EngineType: types.Engine_MYSQL,
		}),
	}

	finder := mock.GetFinder()
	if finder == nil {
		t.Error("GetFinder() returned nil")
	}
}

func TestWithCatalog_NilCatalog(t *testing.T) {
	opts := &reviewOptions{}

	// Set catalog to something first
	mock := &mockCatalog{
		finder: catalog.NewEmptyFinder(&catalog.FinderContext{
			EngineType: types.Engine_MYSQL,
		}),
	}
	WithCatalog(mock)(opts)

	// Attempting to set nil catalog
	// This test documents current behavior - may need adjustment based on requirements
	var nilCatalog catalogInterface
	WithCatalog(nilCatalog)(opts)

	// Catalog should be set to nil
	if opts.catalog != nil {
		t.Error("Expected catalog to be nil when passed nil")
	}
}
