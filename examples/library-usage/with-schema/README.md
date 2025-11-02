# Schema-Aware Review Example

This example demonstrates reviewing SQL changes against existing database schema.

## What it does

1. Creates metadata representing an existing database schema
2. Reviews ALTER statements that modify the schema
3. Detects breaking changes and backward compatibility issues

## Running the example

```bash
go run main.go
```

## Schema metadata structure

The example shows how to construct `DatabaseSchemaMetadata`:

```go
schema := &types.DatabaseSchemaMetadata{
    Name: "mydb",
    Schemas: []*types.SchemaMetadata{
        {
            Name: "public",
            Tables: []*types.TableMetadata{
                {
                    Name: "users",
                    Columns: []*types.ColumnMetadata{...},
                    Indexes: []*types.IndexMetadata{...},
                },
            },
        },
    },
}
```

## Use cases

### Migration validation
```go
// Load current schema from database
schema := loadSchemaFromDB(db)

// Review migration script
result, err := r.ReviewWithSchema(ctx, migrationSQL, schema)

if result.HasErrors() {
    return fmt.Errorf("migration contains breaking changes")
}
```

### Backward compatibility checking
Schema-aware rules can detect:
- Dropping columns that are still in use
- Changing column types in incompatible ways
- Removing indexes
- Foreign key constraint violations

## Integration with ORMs

You can extract schema metadata from popular ORMs:
- **GORM**: Use `db.Migrator().ColumnTypes()`
- **sqlx**: Query `INFORMATION_SCHEMA`
- **database/sql**: Custom queries to system tables

## Key benefits

- Catch breaking changes before deployment
- Validate ALTER statements against actual schema
- Ensure backward compatibility
- Prevent production incidents
