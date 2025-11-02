# SQL Reviewer - Go Library

SQL Reviewer provides a comprehensive API for reviewing SQL statements against configurable quality and style rules in your Go applications.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [API Overview](#api-overview)
- [Examples](#examples)
- [Configuration](#configuration)
- [Rule Registration](#rule-registration)
- [Advanced Usage](#advanced-usage)

## Installation

```bash
go get github.com/nsxbet/sql-reviewer-cli
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/nsxbet/sql-reviewer-cli/pkg/reviewer"
    "github.com/nsxbet/sql-reviewer-cli/pkg/types"
    _ "github.com/nsxbet/sql-reviewer-cli/pkg/rules/mysql"  // Register MySQL rules
)

func main() {
    // Create a reviewer for MySQL
    r := reviewer.New(types.Engine_MYSQL)

    // Review SQL
    result, err := r.Review(context.Background(), "CREATE TABLE users (id INT);")
    if err != nil {
        log.Fatal(err)
    }

    // Check results
    if result.HasErrors() {
        fmt.Println("Found errors:")
        for _, advice := range result.FilterByStatus(types.Advice_ERROR) {
            fmt.Printf("  - %s\n", advice.Content)
        }
    }
}
```

## Core Concepts

### 1. Reviewer

The `Reviewer` is the high-level API for SQL review operations. It manages configuration and executes rules.

```go
r := reviewer.New(types.Engine_MYSQL)
result, err := r.Review(ctx, sqlStatements)
```

### 2. Rules

Rules are validation checks organized by category:
- **Naming**: Table/column naming conventions
- **Schema**: Primary keys, foreign keys, indexes
- **Statement**: Query optimization, safety checks
- **System**: Charset, collation, system objects
- **Column**: Data types, constraints, defaults

### 3. Result

Review results contain findings (advices) categorized by severity:
- **ERROR**: Critical issues that should be fixed
- **WARNING**: Suggestions for improvement
- **SUCCESS**: Confirmations or informational messages

### 4. Configuration

Rules can be configured via YAML/JSON files or programmatically:

```go
r := reviewer.New(types.Engine_MYSQL)
r.WithConfig("custom-rules.yaml")
```

## API Overview

### High-Level API (pkg/reviewer)

**Recommended for most use cases** - Simple, ergonomic interface:

```go
// Basic usage
r := reviewer.New(types.Engine_MYSQL)
result, err := r.Review(ctx, sql)

// With custom config
r.WithConfig("rules.yaml")

// With schema context
result, err := r.ReviewWithSchema(ctx, sql, schema)
```

### Low-Level API (pkg/advisor)

**Advanced use cases** - Direct rule execution with full control:

```go
checkCtx := advisor.Context{
    DBType:     types.Engine_MYSQL,
    Statements: sql,
    Rule:       rule,
}
advices, err := advisor.Check(ctx, types.Engine_MYSQL, ruleType, checkCtx)
```

## Examples

Complete examples are available in [`examples/library-usage/`](../examples/library-usage/):

### [Basic Usage](../examples/library-usage/basic/)
Simple review with default configuration:
```bash
cd examples/library-usage/basic && go run main.go
```

### [Custom Configuration](../examples/library-usage/with-config/)
Load rules from YAML file:
```bash
cd examples/library-usage/with-config && go run main.go
```

### [Schema-Aware Review](../examples/library-usage/with-schema/)
Validate against existing database schema:
```bash
cd examples/library-usage/with-schema && go run main.go
```

### [Result Filtering](../examples/library-usage/filtering/)
Advanced result processing and filtering:
```bash
cd examples/library-usage/filtering && go run main.go
```

## Configuration

### YAML Configuration

```yaml
ruleList:
  - type: naming.table
    level: ERROR
    payload:
      format: "^[a-z]+(_[a-z]+)*$"
      maxLength: 64

  - type: table.require-pk
    level: ERROR

  - type: column.comment
    level: WARNING
    payload:
      required: true
      maxLength: 256
```

### Programmatic Configuration

```go
cfg := &config.Config{
    RuleList: []*types.SQLReviewRule{
        {
            Type:   string(advisor.SchemaRuleTableNaming),
            Level:  types.SQLReviewRuleLevel_ERROR,
            Engine: types.Engine_MYSQL,
            Payload: map[string]interface{}{
                "format":    "^[a-z]+(_[a-z]+)*$",
                "maxLength": 64,
            },
        },
    },
}
r := reviewer.New(types.Engine_MYSQL).WithConfigObject(cfg)
```

## Rule Registration

Rules are automatically registered via `init()` functions. Import rule packages with blank imports:

```go
import (
    _ "github.com/nsxbet/sql-reviewer-cli/pkg/rules/mysql"
    // _ "github.com/nsxbet/sql-reviewer-cli/pkg/rules/postgres"  // When available
)
```

### Available Rule Packages

- `pkg/rules/mysql` - 92 MySQL rules covering all aspects of SQL quality

## Advanced Usage

### Context Cancellation

All review operations support context cancellation for timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := r.Review(ctx, sql)
if err == context.DeadlineExceeded {
    // Handle timeout
}
```

### Database Connection

Some rules need database access for EXPLAIN analysis or dry-run checks:

```go
db, _ := sql.Open("mysql", dsn)
result, err := r.Review(ctx, sql, reviewer.WithDriver(db))
```

### Schema Metadata

Provide existing schema for validation against current state:

```go
schema := &types.DatabaseSchemaMetadata{
    Name: "mydb",
    Schemas: []*types.SchemaMetadata{...},
}
result, err := r.ReviewWithSchema(ctx, sql, schema)
```

### Result Processing

```go
// Filter by severity
errors := result.FilterByStatus(types.Advice_ERROR)
warnings := result.FilterByStatus(types.Advice_WARNING)

// Filter by error code
syntaxErrors := result.FilterByCode(int32(types.StatementSyntaxError))

// Quick checks
if result.IsClean() {
    fmt.Println("All checks passed!")
}

if result.HasErrors() {
    os.Exit(1)  // Fail CI/CD pipeline
}
```

### Custom Rules

Implement the `Advisor` interface to create custom rules:

```go
type MyRule struct{}

func (r *MyRule) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
    // Your validation logic
    if violatesRule {
        return []*types.Advice{{
            Status:  types.Advice_ERROR,
            Title:   "Custom rule violation",
            Content: "Description of the issue",
        }}, nil
    }
    return nil, nil
}

// Register the rule
func init() {
    advisor.Register(types.Engine_MYSQL, "custom.my-rule", &MyRule{})
}
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Review SQL
  run: |
    go run cmd/review/main.go migrations/*.sql || exit 1
```

### As a Library

```go
func main() {
    r := reviewer.New(types.Engine_MYSQL)
    result, err := r.Review(context.Background(), sql)

    if err != nil || result.HasErrors() {
        os.Exit(1)
    }
}
```

## API Reference

Full API documentation is available at [pkg.go.dev](https://pkg.go.dev/github.com/nsxbet/sql-reviewer-cli/pkg):

- [`pkg/reviewer`](https://pkg.go.dev/github.com/nsxbet/sql-reviewer-cli/pkg/reviewer) - High-level API
- [`pkg/advisor`](https://pkg.go.dev/github.com/nsxbet/sql-reviewer-cli/pkg/advisor) - Low-level rule engine
- [`pkg/types`](https://pkg.go.dev/github.com/nsxbet/sql-reviewer-cli/pkg/types) - Type definitions
- [`pkg/config`](https://pkg.go.dev/github.com/nsxbet/sql-reviewer-cli/pkg/config) - Configuration management

## Migration from CLI

If you're currently using the CLI and want to integrate the library:

**CLI:**
```bash
sql-reviewer check -e mysql migrations.sql
```

**Library:**
```go
sqlContent, _ := os.ReadFile("migrations.sql")
r := reviewer.New(types.Engine_MYSQL)
result, err := r.Review(ctx, string(sqlContent))
```

## Support

- **Issues**: https://github.com/nsxbet/sql-reviewer-cli/issues
- **Documentation**: [Main README](../README.md)
- **Examples**: [`examples/library-usage/`](../examples/library-usage/)

## License

See [LICENSE](../LICENSE) file for details.
