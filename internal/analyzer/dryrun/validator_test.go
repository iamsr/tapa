package dryrun

import (
	"testing"
)

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		wantErr    bool
	}{
		// Valid identifiers
		{
			name:       "alphanumeric with underscore",
			identifier: "my_schema",
			wantErr:    false,
		},
		{
			name:       "starts with underscore",
			identifier: "_private",
			wantErr:    false,
		},
		{
			name:       "mixed case with numbers",
			identifier: "Schema123",
			wantErr:    false,
		},
		{
			name:       "single character",
			identifier: "a",
			wantErr:    false,
		},
		{
			name:       "uppercase only",
			identifier: "SCHEMA",
			wantErr:    false,
		},
		{
			name:       "multiple underscores",
			identifier: "my_test_schema_v2",
			wantErr:    false,
		},
		{
			name:       "ends with underscore",
			identifier: "schema_",
			wantErr:    false,
		},
		{
			name:       "ends with number",
			identifier: "schema123",
			wantErr:    false,
		},

		// Invalid identifiers
		{
			name:       "empty string",
			identifier: "",
			wantErr:    true,
		},
		{
			name:       "SQL injection attempt",
			identifier: "'; DROP TABLE users--",
			wantErr:    true,
		},
		{
			name:       "starts with number",
			identifier: "1schema",
			wantErr:    true,
		},
		{
			name:       "contains hyphen",
			identifier: "my-schema",
			wantErr:    true,
		},
		{
			name:       "contains space",
			identifier: "my schema",
			wantErr:    true,
		},
		{
			name:       "contains semicolon",
			identifier: "schema;drop",
			wantErr:    true,
		},
		{
			name:       "contains dot",
			identifier: "schema.table",
			wantErr:    true,
		},
		{
			name:       "SQL injection with union",
			identifier: "schema UNION SELECT * FROM users",
			wantErr:    true,
		},
		{
			name:       "contains parentheses",
			identifier: "schema()",
			wantErr:    true,
		},
		{
			name:       "contains single quote",
			identifier: "schema'test",
			wantErr:    true,
		},
		{
			name:       "contains double quote",
			identifier: `schema"test`,
			wantErr:    true,
		},
		{
			name:       "contains asterisk",
			identifier: "schema*",
			wantErr:    true,
		},
		{
			name:       "contains percent sign",
			identifier: "schema%",
			wantErr:    true,
		},
		{
			name:       "contains backslash",
			identifier: `schema\test`,
			wantErr:    true,
		},
		{
			name:       "contains dollar sign",
			identifier: "schema$1",
			wantErr:    true,
		},
		{
			name:       "contains at symbol",
			identifier: "schema@test",
			wantErr:    true,
		},
		{
			name:       "contains newline",
			identifier: "schema\ntest",
			wantErr:    true,
		},
		{
			name:       "contains tab",
			identifier: "schema\ttest",
			wantErr:    true,
		},
		{
			name:       "contains carriage return",
			identifier: "schema\rtest",
			wantErr:    true,
		},
		{
			name:       "SQL comment injection",
			identifier: "schema--comment",
			wantErr:    true,
		},
		{
			name:       "SQL block comment injection",
			identifier: "schema/*comment*/",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIdentifier(tt.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIdentifier(%q) error = %v, wantErr %v",
					tt.identifier, err, tt.wantErr)
			}
		})
	}
}
