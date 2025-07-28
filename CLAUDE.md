# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the SQL Reviewer CLI project.

## Project Overview

The SQL Reviewer CLI is a command-line tool for reviewing SQL statements against configurable rules. It supports multiple database engines with comprehensive rule sets for ensuring SQL code quality and consistency.

## Project Architecture

- **`cmd/`**: CLI command implementations using Cobra framework
- **`pkg/advisor/`**: Core advisor logic, rule registration, and utility functions
- **`pkg/config/`**: Configuration management for rules and schema integration
- **`pkg/rules/`**: Database engine-specific rule implementations
  - **`pkg/rules/mysql/`**: Complete MySQL rule implementations (92 rules)
- **`pkg/catalog/`**: Database schema catalog and metadata handling
- **`pkg/mysqlparser/`**: ANTLR-based MySQL SQL parser and utilities
- **`pkg/types/`**: Type definitions and data structures
- **`pkg/logger/`**: Logging abstraction layer
- **`schema.yaml`**: Default rule configurations and payload definitions
- **`examples/`**: Example configurations and test SQL files
- **`docs/`**: Project documentation and rule references

## Development Workflow

**ALWAYS follow these steps after making code changes:**

### After Go Code Changes
1. **Format**: Run `gofmt -w` on modified files
2. **Lint**: Run `golangci-lint run --allow-parallel-runners` to catch issues
   - **Important**: Run golangci-lint repeatedly until there are no issues
3. **Auto-fix**: Use `golangci-lint run --fix --allow-parallel-runners` to fix issues automatically
4. **Test**: Run relevant tests before committing: `go test ./...`
5. **Build**: Test build with `go build -o sql-reviewer main.go`

### Rule Development
When implementing new rules:
1. **Follow existing patterns** in `pkg/rules/mysql/` for consistency
2. **Use the BaseAntlrRule framework** for ANTLR-based parsing
3. **Implement proper payload handling** using advisor utility functions
4. **Add comprehensive test data** in corresponding `testdata/` directory
5. **Register the rule** in the appropriate `init.go` file
6. **Update schema.yaml** with default payload configuration if needed

### Configuration Changes
1. **Test schema.yaml** integration with `go run main.go check -e mysql examples/test.sql --debug`
2. **Verify payload normalization** works correctly for YAML config files
3. **Ensure no WARN logs** appear during rule execution

## Build/Test Commands

- **Build CLI**: `go build -o sql-reviewer main.go`
- **Run with debug**: `go run main.go check -e mysql examples/test.sql --debug`
- **Run tests**: `go test ./...`
- **Run specific test**: `go test -v -run TestSpecificFunction ./pkg/...`
- **Lint code**: `golangci-lint run --allow-parallel-runners`
- **Format code**: `gofmt -w .`

## Code Style

- **General**: Follow Go best practices and standard library conventions
- **Conciseness**: Write clean, minimal code; fewer lines is better
- **Comments**: Only include comments that explain non-obvious business logic
- **Error Handling**: Use standard Go error handling with descriptive messages
- **Naming**: Use clear, descriptive names; avoid abbreviations
- **Imports**: Group and sort imports (standard, third-party, local)
- **Testing**: Include test data in `testdata/` directories using YAML format

## Rule Implementation Guidelines

### Rule Structure
```go
type MyRuleAdvisor struct {
    *mysql.BaseAntlrRule
}

func (r *MyRuleAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
    // Parse payload using advisor utilities
    payload, err := advisor.UnmarshalStringTypeRulePayload(rule.Payload)
    if err != nil {
        return nil, err
    }
    
    // Use ANTLR parser for SQL analysis
    res, err := mysql.ParseMySQL(statements)
    if err != nil {
        return nil, err
    }
    
    // Implement rule logic
    checker := &myRuleChecker{
        rule:    rule,
        payload: payload,
    }
    
    return mysql.NewGenericAntlrChecker(res.Tree, checker).Check()
}
```

### Payload Types
- **StringTypeRulePayload**: Use for single string configuration (naming patterns, etc.)
- **StringArrayTypeRulePayload**: Use for lists (allowed/disallowed items)
- **NumberTypeRulePayload**: Use for numeric limits and thresholds
- **BooleanTypeRulePayload**: Use for enable/disable flags

### Error Handling
- Return descriptive error messages with context
- Use proper error wrapping with `fmt.Errorf("context: %w", err)`
- Include line/column information when available
- Provide actionable advice in error messages

## Schema Integration

The project uses `schema.yaml` for default rule configurations:
- **componentList**: Defines rule payload structure
- **Payload types**: STRING, NUMBER, STRING_ARRAY, TEMPLATE, BOOLEAN
- **Multi-component rules**: Naming rules use only the "format" component
- **Engine-specific**: Rules are filtered by engine type (MYSQL, POSTGRES)

## Testing

- **Unit tests**: Test individual rule implementations
- **Integration tests**: Test CLI end-to-end functionality
- **Test data**: Use YAML files in `testdata/` directories
- **Coverage**: Aim for comprehensive coverage of rule logic
- **Performance**: Test with large SQL files when relevant

## Debugging

- Use `--debug` flag for detailed logging
- Check payload parsing with debug output
- Verify rule registration in init functions
- Test ANTLR parsing with complex SQL statements

## Contributing

1. **Follow existing patterns** and architectural decisions
2. **Write comprehensive tests** for new functionality
3. **Update documentation** for new features or rules
4. **Ensure backward compatibility** when possible
5. **Use conventional commit format** for commit messages

## Engine Support

- **MySQL**: Fully implemented with 92 rules
- **PostgreSQL**: Framework ready for implementation
- **Future engines**: Follow MySQL patterns for consistency

## Performance Considerations

- Rules execute sequentially per SQL statement
- ANTLR parsing is cached per statement
- Large SQL files should be processed efficiently
- Memory usage is optimized for typical SQL file sizes

This project provides a robust foundation for SQL quality enforcement across different database engines.