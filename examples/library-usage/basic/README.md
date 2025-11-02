# Basic Library Usage Example

This example demonstrates the simplest way to use SQL Reviewer as a Go library.

## What it does

1. Creates a reviewer with default MySQL rules
2. Reviews a CREATE TABLE statement
3. Displays any errors or warnings found

## Running the example

```bash
go run main.go
```

## Key concepts

- **Default configuration**: Uses built-in rules without loading a config file
- **Rule registration**: Rules are registered via blank imports (`import _ "..."`)
- **Result processing**: Use `result.IsClean()`, `result.HasErrors()`, etc. for easy checking

## Expected output

The example SQL has several common issues that will be detected:
- Missing primary key
- Column without comments (if enabled)
- Table without comment (if enabled)

The output will show these findings categorized by severity.
