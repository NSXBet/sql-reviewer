# DML Dry-Run Validation Example

This example demonstrates how to use the `statement.dml-dry-run` SQL review rule to validate DML statements (INSERT, UPDATE, DELETE) by executing EXPLAIN queries against a live database connection.

## Overview

The DML dry-run rule provides **semantic validation beyond syntax checking** by running `EXPLAIN <statement>` against your database. This catches real-world issues like:

- Missing tables or columns
- Permission/privilege errors
- Type mismatches
- Constraint violations
- Invalid function calls
- Schema inconsistencies

## Features

- ✅ **Transaction Safety**: All EXPLAIN queries wrapped in rolled-back transactions
- ✅ **Performance Limit**: Maximum 100 EXPLAIN queries per review session
- ✅ **Graceful Degradation**: Skip validation if database connection unavailable (no errors)
- ✅ **SET ROLE Support**: PostgreSQL tracks and pre-executes SET ROLE statements
- ✅ **Multi-Engine**: Works with PostgreSQL and MySQL

## Prerequisites

### PostgreSQL Setup

```bash
# Install PostgreSQL driver (if not already installed)
go get github.com/lib/pq

# Set connection string
export POSTGRES_URL='postgres://username:password@localhost:5432/dbname?sslmode=disable'

# Create test database and tables (optional)
psql -U username -d dbname -c "
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(255) UNIQUE
);
"
```

### MySQL Setup

```bash
# Install MySQL driver (if not already installed)
go get github.com/go-sql-driver/mysql

# Set connection string
export MYSQL_DSN='username:password@tcp(localhost:3306)/dbname'

# Create test database and tables (optional)
mysql -u username -p dbname -e "
CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100),
    email VARCHAR(255) UNIQUE
);
"
```

## Running the Example

### With Database Connection (Full Validation)

```bash
# PostgreSQL
export POSTGRES_URL='postgres://user:pass@localhost/dbname'
go run main.go

# MySQL
export MYSQL_DSN='user:pass@tcp(localhost:3306)/dbname'
go run main.go
```

### Without Database Connection (Graceful Skip)

```bash
# DML dry-run will gracefully skip validation
# No environment variables needed
go run main.go
```

## Example Output

### With Valid Database Connection

```
=== SQL Reviewer: DML Dry-Run Validation Example ===

Example 1: PostgreSQL DML Dry-Run with Database Connection
-----------------------------------------------------------
✓ Connected to PostgreSQL

❌ Errors:
  • "INSERT INTO nonexistent_table..." dry runs failed: relation "nonexistent_table" does not exist
    at line 2
```

### Without Database Connection

```
Example 3: Graceful Skip (No Database Connection)
--------------------------------------------------
✓ DML dry-run rule gracefully skipped (no database connection)
✓ No errors reported - validation requires database connection
✓ Clean result (as expected)
```

## Code Examples

### Basic Usage with Database Connection

```go
import (
    "database/sql"
    _ "github.com/lib/pq"
    "github.com/nsxbet/sql-reviewer/pkg/reviewer"
    "github.com/nsxbet/sql-reviewer/pkg/types"
)

// Connect to database
db, _ := sql.Open("postgres", "postgres://user:pass@localhost/dbname")
defer db.Close()

// Create reviewer with database connection
r := reviewer.New(types.Engine_POSTGRES, reviewer.WithDriver(db))

// Enable DML dry-run rule
config := &types.SQLReviewConfig{
    Rules: []*types.SQLReviewRule{
        {
            Type:  string(types.SchemaRuleName_STATEMENT_DML_DRY_RUN),
            Level: types.SQLReviewRuleLevel_ERROR,
        },
    },
}
r.SetConfig(config)

// Review SQL with DML statements
sql := `
INSERT INTO users (id, name) VALUES (1, 'test');
UPDATE users SET email = 'test@example.com' WHERE id = 1;
`

result, _ := r.Review(context.Background(), sql)
```

### PostgreSQL with SET ROLE

```go
// SQL with SET ROLE (automatically tracked and pre-executed)
sql := `
SET ROLE 'app_user';
INSERT INTO users (id, name) VALUES (1, 'test');
UPDATE users SET name = 'updated' WHERE id = 1;
`

// The SET ROLE statement is tracked and executed before each EXPLAIN query
result, _ := r.Review(context.Background(), sql)
```

### Graceful Skip (No Database)

```go
// Create reviewer WITHOUT database connection
r := reviewer.New(types.Engine_POSTGRES)

// Enable DML dry-run rule
config := &types.SQLReviewConfig{
    Rules: []*types.SQLReviewRule{
        {
            Type:  string(types.SchemaRuleName_STATEMENT_DML_DRY_RUN),
            Level: types.SQLReviewRuleLevel_ERROR,
        },
    },
}
r.SetConfig(config)

// DML statements are parsed but not validated (no EXPLAIN queries)
sql := `INSERT INTO users (id, name) VALUES (1, 'test');`
result, _ := r.Review(context.Background(), sql)
// result.IsClean() == true (no validation errors)
```

## How It Works

### Transaction-Based Safety

All EXPLAIN queries are wrapped in a transaction that automatically rolls back:

```go
tx, _ := db.Begin()
defer tx.Rollback()  // Always rolled back - no changes committed

// Execute EXPLAIN query
rows, _ := tx.QueryContext(ctx, "EXPLAIN INSERT INTO users ...")

// Transaction automatically rolled back on defer
```

### Performance Limiting

The rule enforces a maximum of 100 EXPLAIN queries per review session to prevent performance issues with large scripts:

```go
const MaximumLintExplainSize = 100

if explainCount >= MaximumLintExplainSize {
    return // Skip remaining statements
}
```

### SET ROLE Support (PostgreSQL)

PostgreSQL implementation tracks SET ROLE statements and pre-executes them before EXPLAIN queries:

```go
// Detected: SET ROLE 'app_user';
setRoles := []string{"SET ROLE 'app_user';"}

// Before each EXPLAIN:
for _, setRole := range setRoles {
    tx.ExecContext(ctx, setRole)  // Pre-execute SET ROLE
}
tx.QueryContext(ctx, "EXPLAIN INSERT INTO users ...")
```

## Troubleshooting

### Connection Errors

```
Failed to connect to PostgreSQL: dial tcp: lookup localhost: no such host
```

**Solution**: Verify database is running and connection string is correct:
```bash
psql -U user -d dbname  # PostgreSQL
mysql -u user -p dbname  # MySQL
```

### Permission Errors

```
"INSERT INTO users..." dry runs failed: permission denied for table users
```

**Solution**: Grant required permissions to database user:
```sql
-- PostgreSQL
GRANT SELECT, INSERT, UPDATE, DELETE ON users TO username;

-- MySQL
GRANT SELECT, INSERT, UPDATE, DELETE ON dbname.users TO 'username'@'%';
```

### Missing Driver

```
panic: sql: unknown driver "postgres"
```

**Solution**: Import driver in your code:
```go
import _ "github.com/lib/pq"          // PostgreSQL
import _ "github.com/go-sql-driver/mysql"  // MySQL
```

## Performance Considerations

- Each EXPLAIN query requires a database round-trip
- Maximum 100 EXPLAIN queries per review session
- Consider using connection pooling for high-volume reviews
- EXPLAIN queries are read-only and safe for production databases

## See Also

- [Bytebase DML Dry-Run Implementation](../../../docs/bytebase-dml-dry-run-implementation.md)
- [SQL Reviewer Documentation](../../../README.md)
- [PostgreSQL EXPLAIN Documentation](https://www.postgresql.org/docs/current/sql-explain.html)
- [MySQL EXPLAIN Documentation](https://dev.mysql.com/doc/refman/8.0/en/explain.html)
