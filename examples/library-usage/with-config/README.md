# Configuration-Based Usage Example

This example demonstrates how to use custom rule configurations with SQL Reviewer.

## What it does

1. Loads rules from a YAML configuration file
2. Reviews SQL against the configured rules
3. Demonstrates customizing rule severity and payloads

## Running the example

```bash
# Use the included config
go run main.go

# Use a custom config file
go run main.go /path/to/your/rules.yaml
```

## Configuration structure

The `rules.yaml` file shows how to:
- Set rule severity levels (ERROR, WARNING, DISABLED)
- Configure rule-specific payloads (formats, limits, etc.)
- Enable/disable specific rules

## Rule payload examples

### Naming rules
```yaml
- type: naming.table
  level: ERROR
  payload:
    format: "^[a-z]+(_[a-z]+)*$"  # Regex pattern
    maxLength: 64
```

### Limit rules
```yaml
- type: index.key-number-limit
  level: ERROR
  payload:
    number: 5  # Maximum number of columns in an index
```

### Boolean rules
```yaml
- type: column.comment
  level: WARNING
  payload:
    required: true
    maxLength: 256
```

## Key concepts

- **Rule levels**: ERROR (must fix), WARNING (should fix), DISABLED (skip)
- **Payload configuration**: Each rule type has specific configuration options
- **Fallback**: If config loading fails, falls back to default configuration
