# PostgreSQL Syntax Error to Advice Conversion Example

This example demonstrates how PostgreSQL syntax errors are converted to actionable `Advice` objects with error code 201 (StatementSyntaxError), making them visible to users instead of being silently discarded.

## Overview

**Problem**: Previously, PostgreSQL syntax errors were returned as Go errors and silently logged/discarded by the reviewer loop, making them invisible to users.

**Solution**: All 51 PostgreSQL rules now convert syntax errors to `Advice` objects (matching MySQL behavior), ensuring users receive actionable feedback with line/column positions.

## Features Demonstrated

- ✅ Syntax errors converted to `Advice` objects (code 201)
- ✅ Position information preserved (line/column from parser)
- ✅ Error messages included in advice content
- ✅ Consistent behavior with MySQL implementation
- ✅ No silent error discarding - all syntax errors visible

## Running the Example

```bash
# Run the example
cd examples/library-usage/syntax-errors
go run main.go
```

## Expected Output

```
PostgreSQL Syntax Error to Advice Conversion Demo
==================================================

Example 1: Missing table name
SQL: CREATE TABLE;
  ✅ Syntax error converted to advice (Code 201):
     Title: Syntax error
     Content: no viable alternative at input 'CREATE TABLE;'
     Position: Line 1, Column 13
     Status: ERROR

Example 2: Invalid INSERT statement
SQL: INSERT users VALUES (1, 'John');
  ✅ Syntax error converted to advice (Code 201):
     Title: Syntax error
     Content: missing 'INTO' at 'users'
     Position: Line 1, Column 8
     Status: ERROR

Example 3: Missing table in SELECT FROM
SQL: SELECT * FROM WHERE id = 1;
  ✅ Syntax error converted to advice (Code 201):
     Title: Syntax error
     Content: mismatched input 'WHERE' expecting ...
     Position: Line 1, Column 15
     Status: ERROR

Example 4: Incomplete ALTER TABLE
SQL: ALTER TABLE ADD;
  ✅ Syntax error converted to advice (Code 201):
     Title: Syntax error
     Content: no viable alternative at input 'ALTER TABLE ADD;'
     Position: Line 1, Column 16
     Status: ERROR

Example 5: Valid SQL (no syntax errors)
SQL: CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(100));
  ✅ No syntax errors - SQL is valid

==================================================
Key Takeaways:
- Syntax errors are converted to Advice objects (code 201)
- Position information (line/column) is preserved
- Errors are NOT silently discarded - users see them
- Consistent behavior across MySQL and PostgreSQL
```

## Code Explanation

### Key Components

1. **Reviewer Creation**
   ```go
   r := reviewer.New(types.Engine_POSTGRES)
   ```

2. **Review SQL with Syntax Error**
   ```go
   result, err := r.Review(context.Background(), "CREATE TABLE;")
   // err is nil - syntax errors are converted to advice
   ```

3. **Filter Syntax Errors**
   ```go
   syntaxErrors := result.FilterByCode(201) // 201 = StatementSyntaxError
   ```

4. **Access Position Information**
   ```go
   if advice.StartPosition != nil {
       line := advice.StartPosition.Line
       column := advice.StartPosition.Column
   }
   ```

## Error Code Reference

- **Code 201**: `StatementSyntaxError` - SQL syntax is invalid
- **Code 1**: `Internal` - Generic/unexpected error (fallback)

## Architecture

### Before (Broken Behavior)
```
PostgreSQL Rule → getANTLRTree() → Parser Error
                                         ↓
                                    return nil, err
                                         ↓
                             Reviewer Loop (discards error)
                                         ↓
                              User sees nothing ❌
```

### After (Fixed Behavior)
```
PostgreSQL Rule → getANTLRTree() → Parser Error
                                         ↓
                          ConvertSyntaxErrorToAdvice(err)
                                         ↓
                            return []*types.Advice{...}, nil
                                         ↓
                              Reviewer Loop (processes advice)
                                         ↓
                            User sees advice with position ✅
```

## Implementation Pattern

All 51 PostgreSQL rules follow this pattern:

```go
func (*SomeAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
    // Parse SQL and get ANTLR tree
    tree, err := getANTLRTree(checkCtx)
    if err != nil {
        // Convert syntax error to advice (code 201)
        return ConvertSyntaxErrorToAdvice(err)
    }

    // Continue with rule-specific logic...
    // ...
}
```

## Consistency with MySQL

This implementation matches the exact pattern used in MySQL rules (`pkg/rules/mysql/framework.go`), ensuring consistent error handling across all database engines supported by sql-reviewer.

## Testing

Comprehensive tests verify the behavior:
- `pkg/rules/postgres/framework_test.go` - Unit tests for conversion function
- `pkg/rules/postgres/postgres_test.go` - Integration tests for end-to-end flow

Run tests:
```bash
cd ../../..
go test ./pkg/rules/postgres/... -v
```

## Related Files

- **Implementation**: `pkg/rules/postgres/framework.go` (lines 162-217)
- **Script**: `scripts/refactor-postgres-syntax-errors.sh` (automation tool)
- **SQL Examples**: `examples/postgres-syntax-errors.sql`
- **Tests**: `pkg/rules/postgres/framework_test.go`, `postgres_test.go`

## Learn More

- See `pkg/README.md` for complete library documentation
- See `.todos/feature-postgres-syntax-error-advice-plan.md` for implementation details
- See Bytebase implementation: `backend/plugin/advisor/mysql/advisor_*` (reference)
