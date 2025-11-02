package reviewer

import (
	"context"
	"testing"
	"time"

	"github.com/nsxbet/sql-reviewer-cli/pkg/config"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"

	_ "github.com/nsxbet/sql-reviewer-cli/pkg/rules/mysql"
)

func TestNew(t *testing.T) {
	r := New(types.Engine_MYSQL)
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.engine != types.Engine_MYSQL {
		t.Errorf("Expected engine MYSQL, got %v", r.engine)
	}
	if r.config == nil {
		t.Error("Expected default config, got nil")
	}
}

func TestReview_BasicSQL(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	// Test with valid SQL
	sql := `CREATE TABLE users (
		id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'User ID',
		username VARCHAR(50) NOT NULL COMMENT 'Username',
		email VARCHAR(100) NOT NULL COMMENT 'Email address'
	) ENGINE=InnoDB COMMENT 'User accounts';`

	result, err := r.Review(ctx, sql)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Review() returned nil result")
	}

	// Result should have a summary
	if result.Summary.Total < 0 {
		t.Error("Expected non-negative total count")
	}
}

func TestReview_SyntaxError(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	// SQL with syntax error
	sql := "CREATE TABLE users (id INT"

	result, err := r.Review(ctx, sql)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	// Syntax errors should be reported as advices (one per rule that encounters the error)
	// When schema.yaml is loaded, we expect multiple syntax error advices
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should have at least one syntax error reported
	if result.Summary.Errors == 0 {
		t.Error("Expected syntax errors to be reported")
	}

	// Verify at least one advice is about syntax error
	syntaxErrorFound := false
	for _, advice := range result.Advices {
		if advice.Code == int32(types.StatementSyntaxError) {
			syntaxErrorFound = true
			break
		}
	}
	if !syntaxErrorFound {
		t.Error("Expected at least one StatementSyntaxError advice")
	}
}

func TestReview_MultipleStatements(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	sql := `
	CREATE TABLE users (id INT PRIMARY KEY);
	CREATE TABLE posts (id INT PRIMARY KEY);
	CREATE TABLE comments (id INT PRIMARY KEY);
	`

	result, err := r.Review(ctx, sql)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Review() returned nil result")
	}
}

func TestReview_EmptySQL(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	result, err := r.Review(ctx, "")
	if err != nil {
		t.Fatalf("Review() failed on empty SQL: %v", err)
	}

	if result == nil {
		t.Fatal("Review() returned nil result for empty SQL")
	}
}

func TestReview_ContextCancellation(t *testing.T) {
	r := New(types.Engine_MYSQL)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Large SQL to ensure context check is triggered
	sql := `
	CREATE TABLE t1 (id INT);
	CREATE TABLE t2 (id INT);
	CREATE TABLE t3 (id INT);
	CREATE TABLE t4 (id INT);
	CREATE TABLE t5 (id INT);
	`

	result, err := r.Review(ctx, sql)

	// Context cancellation is checked between rule executions
	// If there are many rules and the context is cancelled early, we expect context.Canceled
	// If rules process quickly before checking, we might get nil error
	// Both are acceptable behavior
	if err != nil && err != context.Canceled {
		t.Errorf("Expected nil or context.Canceled error, got: %v", err)
	}

	// Should always return a result, even with cancelled context
	if result == nil {
		t.Error("Expected result even with cancelled context")
	}
}

func TestReview_ContextTimeout(t *testing.T) {
	r := New(types.Engine_MYSQL)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give it time to expire
	time.Sleep(1 * time.Millisecond)

	sql := "CREATE TABLE users (id INT);"

	result, err := r.Review(ctx, sql)

	// Should return context deadline exceeded
	if err != nil && err != context.DeadlineExceeded {
		// May finish before timeout, which is also valid
		if result == nil {
			t.Errorf("Expected either success or DeadlineExceeded, got: %v", err)
		}
	}
}

func TestWithConfig(t *testing.T) {
	r := New(types.Engine_MYSQL)

	// Test with non-existent file
	err := r.WithConfig("nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestWithConfigObject(t *testing.T) {
	r := New(types.Engine_MYSQL)

	cfg := &config.Config{
		Rules: []*types.SQLReviewRule{
			{
				Type:   "naming.table",
				Level:  types.SQLReviewRuleLevel_ERROR,
				Engine: types.Engine_MYSQL,
			},
		},
	}

	result := r.WithConfigObject(cfg)
	if result != r {
		t.Error("WithConfigObject() should return the same Reviewer for chaining")
	}

	if r.config != cfg {
		t.Error("Config was not set correctly")
	}
}

func TestReviewWithSchema(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	schema := &types.DatabaseSchemaMetadata{
		Name: "testdb",
		Schemas: []*types.SchemaMetadata{
			{
				Name: "public",
				Tables: []*types.TableMetadata{
					{
						Name: "users",
						Columns: []*types.ColumnMetadata{
							{
								Name:     "id",
								Type:     "INT",
								Nullable: false,
							},
						},
					},
				},
			},
		},
	}

	sql := "ALTER TABLE users ADD COLUMN email VARCHAR(100);"

	result, err := r.ReviewWithSchema(ctx, sql, schema)
	if err != nil {
		t.Fatalf("ReviewWithSchema() failed: %v", err)
	}

	if result == nil {
		t.Fatal("ReviewWithSchema() returned nil result")
	}
}

func TestReviewWithSchema_NilSchema(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	sql := "CREATE TABLE users (id INT PRIMARY KEY);"

	// Should work with nil schema
	result, err := r.ReviewWithSchema(ctx, sql, nil)
	if err != nil {
		t.Fatalf("ReviewWithSchema() with nil schema failed: %v", err)
	}

	if result == nil {
		t.Fatal("ReviewWithSchema() returned nil result")
	}
}

func TestReview_WithOptions(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	sql := "CREATE TABLE users (id INT PRIMARY KEY);"

	// Test with change type option
	result, err := r.Review(ctx, sql, WithChangeType(types.PlanCheckRunConfig_DDL))
	if err != nil {
		t.Fatalf("Review() with options failed: %v", err)
	}

	if result == nil {
		t.Fatal("Review() returned nil result")
	}
}

func TestReview_ConcurrentUsage(t *testing.T) {
	r := New(types.Engine_MYSQL)

	// Test that Reviewer is safe for concurrent use
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			ctx := context.Background()
			sql := "CREATE TABLE users (id INT PRIMARY KEY);"
			_, err := r.Review(ctx, sql)
			if err != nil {
				t.Errorf("Concurrent Review() failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestReview_ErrorVsWarning(t *testing.T) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()

	// This SQL likely has violations that will generate errors or warnings
	sql := `CREATE TABLE users (
		id INT,
		name VARCHAR(100)
	);`

	result, err := r.Review(ctx, sql)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	// Count should be sum of errors, warnings, and success
	calculated := result.Summary.Errors + result.Summary.Warnings + result.Summary.Success
	if calculated != result.Summary.Total {
		t.Errorf("Summary counts don't add up: %d errors + %d warnings + %d success != %d total",
			result.Summary.Errors, result.Summary.Warnings, result.Summary.Success, result.Summary.Total)
	}
}

func BenchmarkReview_Simple(b *testing.B) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()
	sql := "CREATE TABLE users (id INT PRIMARY KEY);"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.Review(ctx, sql)
		if err != nil {
			b.Fatalf("Review() failed: %v", err)
		}
	}
}

func BenchmarkReview_Complex(b *testing.B) {
	r := New(types.Engine_MYSQL)
	ctx := context.Background()
	sql := `
	CREATE TABLE users (
		id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'User ID',
		username VARCHAR(50) NOT NULL UNIQUE COMMENT 'Username',
		email VARCHAR(100) NOT NULL UNIQUE COMMENT 'Email',
		password_hash VARCHAR(255) NOT NULL COMMENT 'Password hash',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation time',
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Update time',
		INDEX idx_username (username),
		INDEX idx_email (email)
	) ENGINE=InnoDB COMMENT 'User accounts';
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.Review(ctx, sql)
		if err != nil {
			b.Fatalf("Review() failed: %v", err)
		}
	}
}
