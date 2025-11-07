# PostgreSQL Configuration Examples

This directory contains comprehensive PostgreSQL rule configuration examples for the SQL Reviewer CLI.

## Available Configurations

### 1. `postgres-all-rules.yaml`

A comprehensive configuration file containing **ALL 55 available PostgreSQL rules**, organized by category:

- **Naming Rules** (7 rules): Table, column, and index naming conventions
- **Statement Rules** (28 rules): SQL statement quality and safety checks
- **Table Rules** (5 rules): Table-level constraints and best practices
- **Column Rules** (11 rules): Column definitions and constraints
- **Index Rules** (5 rules): Index design and optimization
- **System Rules** (3 rules): System-level configurations

Some rules are marked as `DISABLED` if they are not yet fully implemented or are optional.

**Use Case**: Reference documentation, understanding all available rules, or creating custom configurations.

```bash
./sql-reviewer check -e postgres --rules examples/postgres-all-rules.yaml your-file.sql
```

### 2. `postgres-production.yaml`

A practical, production-ready configuration with sensible defaults for maintaining PostgreSQL database quality.

**Features**:
- **Critical Rules (ERROR level)**: Must-fix issues like missing primary keys, unsafe operations
- **Performance & Safety (WARNING level)**: Performance optimizations, safe migration patterns
- **Code Quality (WARNING level)**: Naming conventions, code organization

**Use Case**: Production environments, CI/CD pipelines, team collaboration.

```bash
./sql-reviewer check -e postgres --rules examples/postgres-production.yaml your-file.sql
```

## Rule Categories

### Naming Rules
Enforce consistent naming conventions across your database schema:
- Table names: `snake_case` format
- Column names: `snake_case` format
- Index names: `idx_{table}_{column_list}`
- Primary keys: `pk_{table}_{column_list}` (PostgreSQL only)
- Unique keys: `uk_{table}_{column_list}`
- Foreign keys: `fk_{referencing_table}_{referencing_column}_{referenced_table}_{referenced_column}`

### Statement Rules
Validate SQL statement quality and safety:
- Disallow `SELECT *` to avoid ambiguity
- Require `WHERE` clauses for `UPDATE`/`DELETE`
- Disallow leading `%` in `LIKE` (can't use index)
- PostgreSQL-specific safety rules for migrations
- Limit affected rows and inserted rows
- DML dry-run validation

### Table Rules
Enforce table-level best practices:
- Require primary keys on all tables
- Optional foreign key restrictions
- Table comment conventions
- Partition table restrictions
- Drop naming conventions (safety)

### Column Rules
Validate column definitions:
- Column type restrictions
- NOT NULL enforcement
- Default value requirements
- Volatile function restrictions
- Comment conventions

### Index Rules
Optimize index design:
- No duplicate columns in indexes
- Limit index key count
- Primary key type restrictions
- CREATE INDEX CONCURRENTLY (non-blocking)

### System Rules
System-level configurations:
- Charset and collation allowlists
- Comment length limits

## PostgreSQL-Specific Features

### Safe Migration Patterns

The PostgreSQL rules include several migration-safety checks:

1. **`statement.disallow-add-column-with-default`**: Adding columns with DEFAULT causes table rewrites in older PostgreSQL versions
2. **`statement.add-check-not-valid`**: Use NOT VALID when adding CHECK constraints to avoid full table scans
3. **`statement.add-foreign-key-not-valid`**: Use NOT VALID when adding foreign keys to avoid full table scans
4. **`statement.disallow-add-not-null`**: Adding NOT NULL can cause downtime
5. **`column.default-disallow-volatile`**: Volatile functions like `clock_timestamp()` update each row during ADD COLUMN
6. **`index.create-concurrently`**: Create indexes with CONCURRENTLY to avoid blocking writes

### Type System

PostgreSQL rules understand type families and aliases:
- `int`, `integer`, `int4` are equivalent
- `bigint`, `int8` are equivalent
- `serial` normalizes to `integer`
- `bigserial` normalizes to `bigint`

## Customization Guide

### Creating Your Own Configuration

1. Start with `postgres-production.yaml` as a template
2. Adjust rule levels based on your needs:
   - `ERROR`: Must be fixed before deployment
   - `WARNING`: Should be reviewed but not blocking
   - `DISABLED`: Rule is not checked

3. Customize payloads for specific requirements:

```yaml
# Example: Stricter naming convention
- type: naming.table
  level: ERROR
  payload:
    format: "^tbl_[a-z][a-z0-9]*(_[a-z0-9]+)*$"  # Must start with tbl_
    maxLength: 63
  engine: POSTGRES

# Example: Higher row limit for bulk operations
- type: statement.insert.row-limit
  level: WARNING
  payload:
    number: 10000  # Allow up to 10,000 rows
  engine: POSTGRES
```

### Common Customizations

#### Disable Column Comments Requirement
```yaml
- type: column.comment
  level: DISABLED
  engine: POSTGRES
```

#### Allow More Indexes Per Table
```yaml
- type: index.total-number-limit
  level: WARNING
  payload:
    number: 20  # Default is 10
  engine: POSTGRES
```

#### Require Fully Qualified Names
```yaml
- type: naming.fully-qualified
  level: ERROR  # Make it required
  engine: POSTGRES
```

## Testing Your Configuration

Test your configuration with example SQL:

```bash
# Create a test SQL file
cat > test.sql <<EOF
CREATE TABLE public.users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL DEFAULT '',
    email VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX CONCURRENTLY idx_users_email ON public.users(email);
EOF

# Run the check
./sql-reviewer check -e postgres --rules examples/postgres-production.yaml test.sql
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: SQL Review
on: [pull_request]
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Review SQL Changes
        run: |
          # Find all SQL files in migrations
          find migrations -name "*.sql" -type f | while read file; do
            ./sql-reviewer check -e postgres --rules config/postgres-rules.yaml "$file"
          done
```

### GitLab CI Example

```yaml
sql-review:
  stage: test
  script:
    - find migrations -name "*.sql" -exec ./sql-reviewer check -e postgres --rules config/postgres-rules.yaml {} \;
```

## Rule Reference

For detailed documentation on each rule, see:
- [Review Rules Documentation](../docs/review-rules.md)
- [PostgreSQL Rule Implementations](../pkg/rules/postgres/)

## Tips and Best Practices

1. **Start Gradually**: Begin with ERROR-level rules only, then gradually add WARNINGs
2. **Team Agreement**: Ensure your team agrees on naming conventions and standards
3. **Document Exceptions**: When you disable rules, document why in your config file
4. **Regular Reviews**: Periodically review and update your rules as your project evolves
5. **Test First**: Always test rule changes on existing migrations before enforcing them

## Troubleshooting

### Common Issues

**Issue**: "Rule not implemented" warnings
- **Solution**: These are rules not yet implemented for PostgreSQL. Set them to `DISABLED` level.

**Issue**: Type mismatch errors (e.g., "bigint not in allowlist")
- **Solution**: PostgreSQL types are lowercase. Use `bigint`, not `BIGINT` in your allowlists.

**Issue**: Rules from `prod.yaml` are still being applied
- **Solution**: Make sure you're using `--rules` flag, not `--config` flag.

## Support

For issues or questions:
- Check the [main README](../README.md)
- Review [Review Rules Documentation](../docs/review-rules.md)
- Open an issue on GitHub
