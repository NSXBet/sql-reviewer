# SQL Reviewer CLI

A command-line tool for reviewing SQL statements against configurable rules. This tool helps ensure SQL code quality and consistency across different database engines.

## Features

- **Complete MySQL Support**: 92 comprehensive rules covering naming conventions, schema constraints, and statement analysis
- **Complete PostgreSQL Support**: 55 comprehensive rules for PostgreSQL best practices and standards
- **Schema.yaml Integration**: Default rule configurations with automatic payload generation
- **Flexible Configuration**: Support for both config/schema.yaml defaults and custom YAML/JSON config files
- **Multiple Output Formats**: Clean text, structured JSON, and YAML output options
- **ANTLR-based Parsing**: Robust SQL parsing with detailed error reporting and line/column information
- **Zero-config Operation**: Works out-of-the-box with sensible defaults for MySQL and PostgreSQL
- **Extensible Architecture**: Modular design supporting easy addition of new database engines and rules

## Installation

### As a CLI Tool

```bash
# Clone the repository
git clone https://github.com/NSXBet/sql-reviewer.git
cd sql-reviewer

# Build the CLI
go build -o sql-reviewer main.go
```

### As a Go Library

```bash
go get github.com/nsxbet/sql-reviewer
```

## Using as a Library

SQL Reviewer can be used as a Go library in your applications. This is the recommended approach for integrating SQL validation into your codebase.

### Quick Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/nsxbet/sql-reviewer/pkg/reviewer"
    "github.com/nsxbet/sql-reviewer/pkg/types"
)

func main() {
    // Create a reviewer for MySQL or PostgreSQL
    r := reviewer.New(types.Engine_MYSQL)
    // r := reviewer.New(types.Engine_POSTGRES) // For PostgreSQL

    // Review SQL statements
    sql := "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100));"
    result, err := r.Review(context.Background(), sql)
    if err != nil {
        log.Fatal(err)
    }

    // Check results
    fmt.Printf("Found %d issues\n", result.Summary.Total)
    if result.HasErrors() {
        for _, advice := range result.FilterByStatus(types.Advice_ERROR) {
            fmt.Printf("ERROR: %s\n", advice.Content)
        }
    }
}
```

### Library Features

- **High-level API**: Simple `reviewer.New()` and `Review()` methods
- **Low-level API**: Direct access to `advisor.Check()` for advanced use cases
- **Context support**: Full support for cancellation and timeouts
- **Custom configuration**: Load rules from YAML/JSON or configure programmatically
- **Schema-aware validation**: Validate against existing database schema
- **Result filtering**: Easy filtering by severity, error code, etc.

### Complete Documentation

See **[pkg/README.md](pkg/README.md)** for complete library documentation including:
- API reference and examples
- Configuration guide
- Schema-aware validation
- Custom rule implementation
- CI/CD integration patterns

### Library Examples

Complete working examples are available in [`examples/library-usage/`](examples/library-usage/):
- **[basic/](examples/library-usage/basic/)** - Simple usage with defaults
- **[with-config/](examples/library-usage/with-config/)** - Custom configuration
- **[with-schema/](examples/library-usage/with-schema/)** - Schema-aware validation
- **[filtering/](examples/library-usage/filtering/)** - Advanced result processing

## Quick Start (CLI)

```bash
# Check a SQL file with default MySQL rules
./sql-reviewer check -e mysql examples/test.sql

# Enable debug output to see detailed rule processing
./sql-reviewer check -e mysql examples/test.sql --debug

# Output results in JSON format
./sql-reviewer check -e mysql -o json examples/test.sql
```

## Usage

### Basic Usage

```bash
# Check a SQL file against MySQL rules (uses config/schema.yaml defaults)
./sql-reviewer check -e mysql migration.sql

# Check with custom rules configuration
./sql-reviewer check -e mysql -r custom-rules.yaml migration.sql

# Check PostgreSQL with JSON output
./sql-reviewer check -e postgres -o json schema.sql
```

### Configuration

The tool supports two configuration approaches:

#### 1. Default Configuration (config/schema.yaml)

The tool includes a comprehensive `config/schema.yaml` file with default rule configurations. No additional setup required:

```bash
# Uses built-in config/schema.yaml automatically
./sql-reviewer check -e mysql your-file.sql
```

#### 2. Custom Configuration File

Create a custom rules configuration file in YAML format:

```yaml
id: "custom-mysql-rules"
rules:
  - type: "naming.table"
    level: "ERROR"
    engine: "MYSQL"
    payload:
      format: "^[a-z][a-z0-9_]*$"
      maxLength: 63
  
  - type: "statement.select.no-select-all"
    level: "WARNING"
    engine: "MYSQL"
    payload: {}
  
  - type: "table.require-pk"
    level: "ERROR"
    engine: "MYSQL"
    payload: {}
```

### Schema-Aware Analysis

Provide database schema information in JSON format to enable advanced schema-aware checks:

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

Usage with schema file:
```bash
./sql-reviewer check -e mysql --schema db-schema.json migration.sql
```

### Command Line Options

#### Global Flags
- `--config`: Configuration file path
- `--verbose`: Enable verbose output  
- `--debug`: Enable debug output (shows rule processing details)

#### Check Command Flags
- `-e, --engine`: Database engine (`mysql`, `postgres`)
- `-o, --output`: Output format (`text`, `json`, `yaml`)
- `-r, --rules`: Path to custom rules configuration file
- `--schema`: Path to database schema file (JSON)
- `--fail-on-error`: Exit with non-zero code if errors are found
- `--fail-on-warning`: Exit with non-zero code if warnings are found

## Examples

### Check MySQL Migration with Default Rules
```bash
./sql-reviewer check -e mysql migration.sql
```

### Check with Custom Rules and Schema
```bash
./sql-reviewer check -e mysql -r custom-rules.yaml --schema db-schema.json migration.sql
```

### CI/CD Integration
```bash
# Fail the build if any errors are found
./sql-reviewer check -e mysql --fail-on-error migration.sql

# JSON output for parsing by other tools
./sql-reviewer check -e mysql -o json migration.sql | jq '.advices[] | select(.status == "ERROR")'
```

## Supported Database Engines

- **MySQL** (`mysql`) - ✅ Complete implementation (92 rules)
- **PostgreSQL** (`postgres`, `postgresql`) - ✅ Complete implementation (55 rules)

## Rule Categories

The tool supports comprehensive SQL review rules organized into categories:

### MySQL Rules (92 total)

<details>
<summary>Click to expand MySQL rules</summary>

#### Naming Conventions (7 rules)
- Table naming patterns
- Column naming patterns
- Index naming (UK, IDX, FK)
- Auto-increment column naming
- Identifier keyword restrictions

#### Schema Rules (25 rules)
- Required columns and primary keys
- Column constraints (NOT NULL, DEFAULT, etc.)
- Data type restrictions and limits
- Character set and collation requirements
- Index and foreign key constraints

#### Statement Rules (45 rules)
- SELECT statement best practices
- WHERE clause requirements and restrictions
- JOIN limitations and performance rules
- DML/DDL operation constraints
- Transaction and execution limits

#### Engine-Specific Rules (15 rules)
- MySQL storage engine requirements (InnoDB)
- Character set and collation allowlists
- System object creation restrictions
- Performance and optimization rules

</details>

### PostgreSQL Rules (55 total)

#### Naming Conventions (10 rules)
- Table naming patterns with regex support
- Column naming patterns
- Index naming conventions (PK, UK, FK, IDX)
- Fully qualified object names
- DROP TABLE naming patterns
- Table and column comment requirements

#### Column Rules (10 rules)
- Required columns enforcement
- NOT NULL constraints
- Default value requirements
- Column type change restrictions
- Disallowed column types
- Maximum character length limits
- Volatile default value restrictions
- Auto-increment constraints (PostgreSQL SERIAL types)
- Default values for NOT NULL columns

#### Index Rules (7 rules)
- No duplicate columns in indexes
- Index key number limits
- Primary key type restrictions
- Total index count limits
- Concurrent index creation requirements
- Primary key requirements
- Foreign key restrictions

#### Statement Rules (18 rules)
- SELECT * restrictions
- WHERE clause requirements for SELECT/UPDATE/DELETE
- Leading wildcard LIKE restrictions
- INSERT column specification requirements
- INSERT row count limits
- ORDER BY RANDOM() restrictions
- COMMIT statement restrictions
- LIMIT value constraints
- Merge ALTER TABLE recommendations
- ADD COLUMN with DEFAULT restrictions
- CHECK constraint NOT VALID requirements
- Foreign key NOT VALID requirements
- ADD NOT NULL restrictions
- Schema specification requirements
- Mixed DDL/DML restrictions
- Affected row limits

#### Table & System Rules (10 rules)
- Partitioned table restrictions
- ON DELETE CASCADE restrictions
- DROP TABLE CASCADE restrictions
- Character set allowlists
- Collation allowlists
- DML dry-run validation
- Prior backup checks
- Non-transactional statement detection
- SET ROLE variable checks
- Object ownership validation

### Rule Examples

#### MySQL Examples
```sql
-- ❌ Fails naming.table rule
CREATE TABLE UserData (id INT);

-- ✅ Passes naming.table rule
CREATE TABLE user_data (id INT);

-- ❌ Fails table.require-pk rule
CREATE TABLE logs (message TEXT);

-- ✅ Passes table.require-pk rule
CREATE TABLE logs (id INT PRIMARY KEY, message TEXT);
```

#### PostgreSQL Examples
```sql
-- ❌ Fails naming.column rule
CREATE TABLE book (id INT, "creatorId" INT);

-- ✅ Passes naming.column rule
CREATE TABLE book (id INT, creator_id INT);

-- ❌ Fails statement.add-fk-not-valid rule
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);

-- ✅ Passes statement.add-fk-not-valid rule
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;

-- ❌ Fails statement.select.no-select-all rule
SELECT * FROM users WHERE id = 1;

-- ✅ Passes statement.select.no-select-all rule
SELECT id, name, email FROM users WHERE id = 1;
```

## Architecture

The tool follows a modular, extensible architecture:

```
sql-reviewer-cli/
├── cmd/                    # CLI command implementations (Cobra)
├── pkg/
│   ├── advisor/           # Core rule engine and registration
│   ├── config/            # Configuration handling and schema.yaml
│   ├── rules/
│   │   ├── mysql/         # MySQL-specific rule implementations (92 rules)
│   │   └── postgres/      # PostgreSQL-specific rule implementations (55 rules)
│   ├── catalog/           # Database schema metadata handling
│   ├── mysqlparser/       # ANTLR-based MySQL SQL parser
│   ├── pgparser/          # ANTLR-based PostgreSQL SQL parser
│   ├── types/             # Shared type definitions
│   └── logger/            # Logging utilities
├── config/
│   └── schema.yaml        # Default rule configurations
├── examples/              # Example configurations and SQL files
└── docs/                  # Documentation
```

### Key Components

- **ANTLR Parser**: Robust SQL parsing with detailed AST analysis
- **Rule Engine**: Generic framework supporting multiple database engines
- **Configuration System**: Flexible YAML/JSON config with schema integration
- **Payload Normalization**: Automatic conversion between config formats
- **Advisor Registration**: Plugin-style rule registration system

## Development

See [CLAUDE.md](CLAUDE.md) for comprehensive development guidelines.

### Adding New Rules

For MySQL:
1. **Create rule implementation** in `pkg/rules/mysql/`
2. **Add test data** in `pkg/rules/mysql/testdata/`
3. **Register the rule** in `pkg/rules/mysql/init.go`
4. **Update config/schema.yaml** with default configuration
5. **Test thoroughly** with various SQL patterns

For PostgreSQL:
1. **Create rule implementation** in `pkg/rules/postgres/`
2. **Add test data** in `pkg/rules/postgres/testdata/`
3. **Register the rule** in `pkg/rules/postgres/init.go`
4. **Create test file** in `pkg/rules/postgres/*_test.go`
5. **Test thoroughly** with various SQL patterns

### Rule Implementation Example

```go
type TableCommentAdvisor struct {
    *mysql.BaseAntlrRule
}

func (r *TableCommentAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.Context) ([]*types.Advice, error) {
    res, err := mysql.ParseMySQL(statements)
    if err != nil {
        return nil, err
    }

    checker := &tableCommentChecker{rule: rule}
    return mysql.NewGenericAntlrChecker(res.Tree, checker).Check()
}
```

## Testing

```bash
# Run all tests
go test ./...

# Run MySQL rule tests specifically
go test ./pkg/rules/mysql/

# Run PostgreSQL rule tests specifically
go test ./pkg/rules/postgres/

# Test with debug output (MySQL)
go run main.go check -e mysql examples/test.sql --debug

# Test with debug output (PostgreSQL)
go run main.go check -e postgres examples/postgres-test.sql --debug
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/new-rule`)
3. Implement your changes following the patterns in [CLAUDE.md](CLAUDE.md)
4. Add comprehensive tests
5. Update documentation as needed
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Roadmap

- [x] **PostgreSQL Engine**: Complete PostgreSQL rule implementation (55 rules)
- [ ] **Performance Optimization**: Parallel rule execution
- [ ] **Rule Management**: CLI commands for listing and validating rules
- [ ] **CI/CD Integration**: GitHub Actions and pipeline examples
- [ ] **VS Code Extension**: IDE integration for real-time SQL review
- [ ] **Additional Engines**: Oracle, SQL Server, and Snowflake support
- [ ] **PostgreSQL Advanced Rules**: Implement remaining 5 database-dependent rules