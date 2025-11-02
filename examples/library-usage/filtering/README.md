# Result Filtering Example

This example demonstrates advanced result processing and filtering capabilities.

## What it does

1. Reviews multiple SQL statements with various issues
2. Filters results by severity (errors vs warnings)
3. Groups findings by error code
4. Demonstrates exit code handling for CI/CD integration

## Running the example

```bash
go run main.go
```

## Key filtering methods

### Filter by status
```go
errors := result.FilterByStatus(types.Advice_ERROR)
warnings := result.FilterByStatus(types.Advice_WARNING)
```

### Filter by error code
```go
syntaxErrors := result.FilterByCode(int32(types.StatementSyntaxError))
```

### Quick checks
```go
if result.HasErrors() {
    os.Exit(1)  // Fail CI/CD pipeline
}

if result.IsClean() {
    fmt.Println("All checks passed!")
}
```

## CI/CD Integration

This example shows how to integrate SQL review into CI/CD pipelines:

```go
// Fail the build if errors are found
if result.HasErrors() {
    log.Fatal("SQL review found errors")
}

// Warn but don't fail on warnings
if result.HasWarnings() {
    log.Println("SQL review found warnings")
}
```

## Custom result processing

The example demonstrates grouping findings by error code, but you can also:
- Group by line number
- Filter by specific rule types
- Create custom severity thresholds
- Generate reports in different formats
