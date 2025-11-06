package pgparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePostgreSQL(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name:    "simple CREATE TABLE",
			sql:     "CREATE TABLE users (id INT);",
			wantErr: false,
		},
		{
			name:    "CREATE TABLE with schema",
			sql:     "CREATE TABLE public.users (id INT, name VARCHAR(100));",
			wantErr: false,
		},
		{
			name:    "SELECT statement",
			sql:     "SELECT * FROM users WHERE id = 1;",
			wantErr: false,
		},
		{
			name:    "syntax error - missing semicolon is ok",
			sql:     "CREATE TABLE users (id INT)",
			wantErr: false,
		},
		{
			name:    "syntax error - invalid SQL",
			sql:     "CREATE INVALID SYNTAX",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePostgreSQL(tt.sql)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.NotNil(t, result.Tree)
				assert.NotNil(t, result.Tokens)
			}
		})
	}
}

func TestToLowerCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase",
			input: "users",
			want:  "users",
		},
		{
			name:  "uppercase",
			input: "USERS",
			want:  "users",
		},
		{
			name:  "mixed case",
			input: "Users",
			want:  "users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toLowerCase(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeSchemaName(t *testing.T) {
	tests := []struct {
		name       string
		schemaName string
		want       string
	}{
		{
			name:       "empty schema defaults to public",
			schemaName: "",
			want:       "public",
		},
		{
			name:       "explicit schema",
			schemaName: "myschema",
			want:       "myschema",
		},
		{
			name:       "public schema",
			schemaName: "public",
			want:       "public",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSchemaName(tt.schemaName)
			assert.Equal(t, tt.want, got)
		})
	}
}
