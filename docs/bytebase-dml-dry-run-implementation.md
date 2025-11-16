# Bytebase PostgreSQL DML Dry-Run Implementation

This document describes how Bytebase implements the `statement.dml-dry-run` SQL review rule for PostgreSQL (and other database engines).

## Overview

Bytebase's DML dry-run check validates DML statements (INSERT, UPDATE, DELETE) by executing **EXPLAIN queries against a live database connection** within a transaction that gets automatically rolled back. This provides semantic validation beyond syntax checking.

## Architecture

### File Locations

- **PostgreSQL Implementation**: `backend/plugin/advisor/pg/advisor_statement_dml_dry_run.go`
- **MySQL Implementation**: `backend/plugin/advisor/mysql/rule_statement_dml_dry_run.go`
- **Query Helper**: `backend/plugin/advisor/utils.go`
- **Configuration**: `frontend/src/types/sql-review-schema.yaml`

### Rule Registration

```go
func init() {
    advisor.Register(storepb.Engine_POSTGRES, advisor.SchemaRuleStatementDMLDryRun, &StatementDMLDryRunAdvisor{})
}
```

## Core Mechanism

### 1. Statement Detection via ANTLR Walker

The rule uses ANTLR's tree walker to detect DML statements at the top level:

```go
type statementDMLDryRunChecker struct {
    *parser.BasePostgreSQLParserListener

    adviceList               []*storepb.Advice
    level                    storepb.Advice_Status
    title                    string
    driver                   *sql.DB
    ctx                      context.Context
    explainCount             int
    setRoles                 []string
    usePostgresDatabaseOwner bool
    statementsText           string
}

// Detects INSERT statements
func (c *statementDMLDryRunChecker) EnterInsertstmt(ctx *parser.InsertstmtContext) {
    if !isTopLevel(ctx.GetParent()) {
        return
    }
    c.checkDMLDryRun(ctx)
}

// Detects UPDATE statements
func (c *statementDMLDryRunChecker) EnterUpdatestmt(ctx *parser.UpdatestmtContext) {
    if !isTopLevel(ctx.GetParent()) {
        return
    }
    c.checkDMLDryRun(ctx)
}

// Detects DELETE statements
func (c *statementDMLDryRunChecker) EnterDeletestmt(ctx *parser.DeletestmtContext) {
    if !isTopLevel(ctx.GetParent()) {
        return
    }
    c.checkDMLDryRun(ctx)
}
```

### 2. EXPLAIN Query Execution

Each detected DML statement is validated using PostgreSQL's EXPLAIN command:

```go
func (c *statementDMLDryRunChecker) checkDMLDryRun(ctx antlr.ParserRuleContext) {
    // Performance limit: only check up to MaximumLintExplainSize statements
    if c.explainCount >= common.MaximumLintExplainSize {
        return
    }
    c.explainCount++

    // Extract the statement text from source
    stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
    normalizedStmt := advisor.NormalizeStatement(stmtText)

    // Run EXPLAIN to perform dry run (validates syntax + semantics)
    _, err := advisor.Query(c.ctx, advisor.QueryContext{
        UsePostgresDatabaseOwner: c.usePostgresDatabaseOwner,
        PreExecutions:            c.setRoles,  // SET ROLE statements if needed
    }, c.driver, storepb.Engine_POSTGRES, fmt.Sprintf("EXPLAIN %s", stmtText))

    if err != nil {
        // Return advice with error details
        c.adviceList = append(c.adviceList, &storepb.Advice{
            Status:  c.level,
            Code:    advisor.StatementDMLDryRunFailed.Int32(),
            Title:   c.title,
            Content: fmt.Sprintf("\"%s\" dry runs failed: %s", normalizedStmt, err.Error()),
            StartPosition: &storepb.Position{
                Line:   int32(ctx.GetStart().GetLine()),
                Column: 0,
            },
        })
    }
}
```

### 3. Transaction-Based Safety

The `advisor.Query()` helper ensures safety by wrapping all queries in a transaction that **always gets rolled back**:

```go
// Query runs the EXPLAIN or SELECT statements for advisors.
func Query(ctx context.Context, qCtx QueryContext, connection *sql.DB, engine storepb.Engine, statement string) ([]any, error) {
    // Start a transaction
    tx, err := connection.BeginTx(ctx, &sql.TxOptions{})
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()  // ✅ ALWAYS ROLLED BACK - no changes committed

    // Optional: Switch to database owner role for permission checks
    if engine == storepb.Engine_POSTGRES && qCtx.UsePostgresDatabaseOwner {
        const query = `
        SELECT
            u.rolname
        FROM
            pg_roles AS u JOIN pg_database AS d ON (d.datdba = u.oid)
        WHERE
            d.datname = current_database();
        `
        var owner string
        if err := tx.QueryRowContext(ctx, query).Scan(&owner); err != nil {
            return nil, err
        }
        if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET ROLE '%s';", owner)); err != nil {
            return nil, err
        }
    }

    // Execute any pre-execution statements (like SET ROLE)
    for _, preExec := range qCtx.PreExecutions {
        if preExec != "" {
            if _, err := tx.ExecContext(ctx, preExec); err != nil {
                return nil, errors.Wrapf(err, "failed to execute pre-execution: %s", preExec)
            }
        }
    }

    // Execute the EXPLAIN query
    rows, err := tx.QueryContext(ctx, statement)  // e.g., "EXPLAIN INSERT INTO users ..."
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // Parse and return results
    // ... (result processing code)

    // Transaction is automatically rolled back on defer
    return data, nil
}
```

**Key Safety Features:**
- Transaction automatically rolled back via `defer tx.Rollback()`
- No changes ever committed to the database
- EXPLAIN command doesn't modify data anyway
- Safe to run against production databases

### 4. SET ROLE Statement Tracking

PostgreSQL dry-run supports capturing `SET ROLE` statements to execute them before the EXPLAIN:

```go
// EnterVariablesetstmt handles SET ROLE statements
func (c *statementDMLDryRunChecker) EnterVariablesetstmt(ctx *parser.VariablesetstmtContext) {
    if !isTopLevel(ctx.GetParent()) {
        return
    }

    // Check if this is SET ROLE
    if ctx.SET() != nil && ctx.Set_rest() != nil && ctx.Set_rest().Set_rest_more() != nil {
        setRestMore := ctx.Set_rest().Set_rest_more()
        if setRestMore.ROLE() != nil {
            // Store the SET ROLE statement text for pre-execution
            stmtText := extractStatementText(c.statementsText, ctx.GetStart().GetLine(), ctx.GetStop().GetLine())
            c.setRoles = append(c.setRoles, stmtText)
        }
    }
}
```

This allows the dry-run to simulate permission checks under different roles.

### 5. Statement Text Extraction

The rule extracts the original SQL text from the source using line numbers:

```go
func extractStatementText(statementsText string, startLine, endLine int) string {
    lines := strings.Split(statementsText, "\n")
    if startLine < 1 || startLine > len(lines) {
        return ""
    }

    // Convert to 0-indexed
    startIdx := startLine - 1
    endIdx := endLine - 1

    if endIdx >= len(lines) {
        endIdx = len(lines) - 1
    }

    // Extract the lines for this statement
    var stmtLines []string
    for i := startIdx; i <= endIdx; i++ {
        stmtLines = append(stmtLines, lines[i])
    }

    return strings.TrimSpace(strings.Join(stmtLines, " "))
}
```

## What EXPLAIN Validates

PostgreSQL's EXPLAIN command validates:

1. **Syntax**: Statement must be valid PostgreSQL SQL
2. **Schema**: Tables, views, and columns must exist
3. **Permissions**: User must have required privileges (SELECT/INSERT/UPDATE/DELETE)
4. **Types**: Data types must match column definitions
5. **Constraints**: Foreign keys, check constraints must be valid
6. **Functions**: Called functions must exist and be accessible
7. **Query Plan**: Statement must be plannable by the query optimizer

## Performance Considerations

### Maximum EXPLAIN Limit

To prevent performance issues with large scripts containing many DML statements:

```go
if c.explainCount >= common.MaximumLintExplainSize {
    return
}
c.explainCount++
```

The constant `common.MaximumLintExplainSize` limits the number of EXPLAIN queries executed per review.

### Conditional Execution

The rule only runs if a database connection is available:

```go
func (*StatementDMLDryRunAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*storepb.Advice, error) {
    // ... setup code ...

    // Only run EXPLAIN queries if we have a database connection
    if checker.driver != nil {
        antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)
    }

    return checker.adviceList, nil
}
```

If `checkCtx.Driver` is `nil`, the rule gracefully skips validation without errors.

## Configuration

### Schema Definition

In `frontend/src/types/sql-review-schema.yaml`:

```yaml
- type: statement.dml-dry-run
  category: STATEMENT
  engine: POSTGRES

- type: statement.dml-dry-run
  category: STATEMENT
  engine: MYSQL

- type: statement.dml-dry-run
  category: STATEMENT
  engine: MARIADB

- type: statement.dml-dry-run
  category: STATEMENT
  engine: TIDB

- type: statement.dml-dry-run
  category: STATEMENT
  engine: ORACLE

- type: statement.dml-dry-run
  category: STATEMENT
  engine: OCEANBASE
```

### Rule Usage

No payload is needed - the rule is boolean (enabled/disabled):

```yaml
rules:
  - type: statement.dml-dry-run
    level: ERROR
    engine: POSTGRES
```

## Advantages

✅ **Real Validation**: Catches schema mismatches (table/column doesn't exist)
✅ **Permission Checks**: Validates user has proper access
✅ **Type Checking**: PostgreSQL validates data types and constraints
✅ **Safe Execution**: Transaction rollback ensures no database changes
✅ **Multi-statement Support**: Handles scripts with multiple DML statements
✅ **Role Simulation**: Supports SET ROLE for permission testing

## Requirements

⚠️ **Live Database Connection Required**: The rule requires `checkCtx.Driver != nil`
⚠️ **Schema Must Exist**: Can't dry-run against future schema (only current)
⚠️ **Network Access**: Needs connectivity to target database
⚠️ **Performance Impact**: Each EXPLAIN query hits the database

## Error Reporting

When a DML statement fails dry-run validation:

```go
advice := &storepb.Advice{
    Status:  c.level,                                    // ERROR or WARNING
    Code:    advisor.StatementDMLDryRunFailed.Int32(),   // Error code
    Title:   c.title,                                    // Rule type
    Content: fmt.Sprintf("\"%s\" dry runs failed: %s", normalizedStmt, err.Error()),
    StartPosition: &storepb.Position{
        Line:   int32(ctx.GetStart().GetLine()),
        Column: 0,
    },
}
```

**Example Error Output:**
```
"INSERT INTO users (id, email) VALUES (1, 'test@example.com')" dry runs failed: relation "users" does not exist
Line: 5
```

## Comparison: MySQL vs PostgreSQL Implementation

### MySQL Implementation

MySQL uses a similar approach but with a different rule structure:

```go
// MySQL uses a rule-based pattern
type StatementDmlDryRunRule struct {
    BaseRule
    driver       *sql.DB
    ctx          context.Context
    explainCount int
}

func (r *StatementDmlDryRunRule) handleStmt(text string, lineNumber int) {
    r.explainCount++
    if _, err := advisor.Query(r.ctx, advisor.QueryContext{}, r.driver, storepb.Engine_MYSQL, fmt.Sprintf("EXPLAIN %s", text)); err != nil {
        r.AddAdvice(&storepb.Advice{
            Status:        r.level,
            Code:          advisor.StatementDMLDryRunFailed.Int32(),
            Title:         r.title,
            Content:       fmt.Sprintf("\"%s\" dry runs failed: %s", text, err.Error()),
            StartPosition: common.ConvertANTLRLineToPosition(r.baseLine + lineNumber),
        })
    }
}
```

**Key Differences:**
- MySQL uses `Rule` pattern with `OnEnter/OnExit` methods
- PostgreSQL uses direct ANTLR listener pattern
- Both use the same `advisor.Query()` transaction mechanism
- Both support the same EXPLAIN-based validation approach

### PostgreSQL Specific Features

1. **Database Owner Role Support**: PostgreSQL can automatically switch to database owner role
2. **SET ROLE Tracking**: PostgreSQL implementation tracks and pre-executes SET ROLE statements
3. **UsePostgresDatabaseOwner Flag**: PostgreSQL-specific context flag for permission elevation

## Testing

The rule is tested via ANTLR-based tests:

```go
// In backend/plugin/advisor/pg/pgantlr_test.go
var antlrRules = []advisor.SQLReviewRuleType{
    // ... other rules ...
    advisor.SchemaRuleStatementDMLDryRun,  // Migrated from legacy
    // ... other rules ...
}

for _, rule := range antlrRules {
    needMetaData := advisorNeedMockData[rule]
    RunANTLRAdvisorRuleTest(t, rule, storepb.Engine_POSTGRES, needMetaData, false /* record */)
}
```

**Note**: `statement.dml-dry-run` is marked as a database-dependent rule, so test infrastructure requires a mock database connection.

## Summary

Bytebase's PostgreSQL DML dry-run check is a **database-dependent rule** that:

1. ✅ Requires a live database connection
2. ✅ Uses `EXPLAIN <statement>` to validate DML queries
3. ✅ Wraps everything in a rolled-back transaction for safety
4. ✅ Captures and reports errors as advice with position information
5. ✅ Supports role-based permission simulation via SET ROLE
6. ✅ Limits the number of EXPLAIN queries for performance
7. ✅ Provides semantic validation beyond syntax checking

This approach catches real-world issues like:
- Missing tables or columns
- Permission/privilege errors
- Type mismatches
- Constraint violations
- Invalid function calls
- Schema inconsistencies

The transaction-based rollback mechanism ensures the validation is completely safe to run against production databases without any risk of data modification.
