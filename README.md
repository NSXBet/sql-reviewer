# SQL Reviewer CLI

A command-line tool for reviewing SQL statements against configurable rules. This tool helps ensure SQL code quality and consistency across different database engines.

## Features

- Support for multiple database engines (MySQL, PostgreSQL, Oracle, SQL Server, Snowflake)
- Configurable rule sets
- Multiple output formats (text, JSON, YAML)
- Schema-aware analysis
- Extensible architecture

## Installation

```bash
go build -o sql-reviewer main.go
```

## Usage

### Basic Usage

```bash
# Check a SQL file against MySQL rules
./sql-reviewer check -e mysql schema.sql

# Check with custom rules configuration
./sql-reviewer check -e postgres -r rules.yaml migration.sql

# Output results in JSON format
./sql-reviewer check -e mysql -o json schema.sql
```

### Configuration

Create a rules configuration file in YAML format:

```yaml
engine: mysql
rules:
  - type: "naming.table"
    level: "ERROR"
    payload: '{"format": "^[a-z][a-z0-9_]*$", "maxLength": 63}'
    engine: "MYSQL"
    comment: "Table names should be lowercase with underscores"
  
  - type: "statement.select.no-select-all"
    level: "WARNING"
    payload: '{}'
    engine: "MYSQL"
    comment: "Avoid using SELECT *"
```

### Schema File

Provide database schema information in JSON format to enable schema-aware checks:

```json
{
  "name": "mydb",
  "schemas": [
    {
      "name": "public",
      "tables": [
        {
          "name": "users",
          "columns": [
            {
              "name": "id",
              "type": "INTEGER",
              "nullable": false
            },
            {
              "name": "email",
              "type": "VARCHAR(255)",
              "nullable": false
            }
          ]
        }
      ]
    }
  ]
}
```

### Command Line Options

#### Global Flags
- `--config`: Configuration file path
- `--verbose`: Enable verbose output
- `--debug`: Enable debug output

#### Check Command Flags
- `-e, --engine`: Database engine (mysql, postgres, oracle, mssql, snowflake)
- `-o, --output`: Output format (text, json, yaml)
- `-r, --rules`: Path to rules configuration file
- `--schema`: Path to database schema file
- `--fail-on-error`: Exit with non-zero code if errors are found
- `--fail-on-warning`: Exit with non-zero code if warnings are found

## Examples

### Check MySQL Migration
```bash
./sql-reviewer check -e mysql -r mysql-rules.yaml --schema db-schema.json migration.sql
```

### Check PostgreSQL with JSON Output
```bash
./sql-reviewer check -e postgres -o json --fail-on-error schema.sql
```

### Custom Configuration File
```bash
./sql-reviewer --config ~/.sql-reviewer.yaml check -e mysql schema.sql
```

## Supported Database Engines

- **MySQL** (`mysql`)
- **PostgreSQL** (`postgres`, `postgresql`)

## Rule Types

The tool supports various types of SQL review rules:

### Naming Conventions
- Table naming conventions
- Column naming conventions  
- Index naming conventions
- Constraint naming conventions

### Schema Rules
- Required columns
- Column nullability
- Data type restrictions
- Primary key requirements

### Statement Rules
- SELECT statement restrictions
- WHERE clause requirements
- JOIN limitations
- Transaction rules

### Engine-Specific Rules
- MySQL storage engine requirements
- PostgreSQL-specific constraints
- Oracle naming restrictions

## Architecture

The tool is designed with a modular architecture:

- `pkg/advisor/`: Core advisor logic and rule definitions
- `pkg/catalog/`: Database schema catalog and lookup functionality
- `pkg/config/`: Configuration management
- `pkg/logger/`: Logging abstraction
- `pkg/types/`: Type definitions and data structures
- `cmd/`: CLI command implementations

## Development

To extend the tool with new rules or database engines:

1. Add new advisor types in `pkg/advisor/types.go`
2. Implement the advisor interface in `pkg/advisor/`
3. Register the advisor in the appropriate engine mapping
4. Add error codes in `pkg/advisor/code.go`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

This project is licensed under the MIT License.