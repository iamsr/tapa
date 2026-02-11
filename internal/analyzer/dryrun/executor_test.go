package dryrun_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/dryrun"
	"github.com/iamsr/tapa/pkg/models"
)

func TestExecutor_ExecuteSQL(t *testing.T) {
	executor := dryrun.NewExecutor("postgresql")

	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name:    "valid syntax",
			sql:     "SELECT 1",
			wantErr: false,
		},
		{
			name:    "invalid syntax",
			sql:     "SELECT FROM WHERE",
			wantErr: true,
		},
		{
			name:    "multiple statements - all valid",
			sql:     "SELECT 1; SELECT 2; SELECT 3",
			wantErr: false,
		},
		{
			name:    "empty SQL",
			sql:     "",
			wantErr: false,
		},
		{
			name:    "whitespace only",
			sql:     "   \n\t  ",
			wantErr: false,
		},
		{
			name:    "invalid pattern - INSERT INTO VALUES",
			sql:     "INSERT INTO VALUES (1, 2, 3)",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := executor.ExecuteSQL(ctx, nil, tt.sql, "test_schema")

			if tt.wantErr && result.ErrorCount == 0 {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && result.ErrorCount > 0 {
				t.Errorf("Expected no error but got %d errors: %v", result.ErrorCount, result.Errors)
			}

			// Verify result structure
			if result.TempSchemaName != "test_schema" {
				t.Errorf("Expected TempSchemaName to be 'test_schema', got '%s'", result.TempSchemaName)
			}
			if !result.RolledBack {
				t.Error("Expected RolledBack to be true")
			}
			if result.ErrorCount != len(result.Errors) {
				t.Errorf("ErrorCount %d doesn't match len(Errors) %d", result.ErrorCount, len(result.Errors))
			}
		})
	}
}

func TestExecutor_ErrorClassification(t *testing.T) {
	executor := dryrun.NewExecutor("postgresql")

	tests := []struct {
		name          string
		sql           string
		expectedType  models.ErrorType
		shouldHaveErr bool
	}{
		{
			name:          "syntax error pattern",
			sql:           "SELECT FROM WHERE",
			expectedType:  models.ErrorTypeSyntaxError,
			shouldHaveErr: true,
		},
		{
			name:          "valid SQL",
			sql:           "SELECT 1",
			expectedType:  "",
			shouldHaveErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := executor.ExecuteSQL(ctx, nil, tt.sql, "test_schema")

			if tt.shouldHaveErr {
				if result.ErrorCount == 0 {
					t.Error("Expected error but got none")
					return
				}
				if result.Status != models.DryRunStatusFailed {
					t.Errorf("Expected status FAILED, got %s", result.Status)
				}
				if result.Errors[0].ErrorType != tt.expectedType {
					t.Errorf("Expected error type %s, got %s", tt.expectedType, result.Errors[0].ErrorType)
				}
			} else {
				if result.ErrorCount > 0 {
					t.Errorf("Expected no errors but got %d", result.ErrorCount)
				}
				if result.Status != models.DryRunStatusSuccess {
					t.Errorf("Expected status SUCCESS, got %s", result.Status)
				}
			}
		})
	}
}

func TestExecutor_MultipleStatements(t *testing.T) {
	executor := dryrun.NewExecutor("postgresql")

	tests := []struct {
		name           string
		sql            string
		wantStmtCount  int
		wantErrorCount int
	}{
		{
			name:           "single statement no semicolon",
			sql:            "SELECT 1",
			wantStmtCount:  1,
			wantErrorCount: 0,
		},
		{
			name:           "single statement with semicolon",
			sql:            "SELECT 1;",
			wantStmtCount:  1,
			wantErrorCount: 0,
		},
		{
			name:           "multiple valid statements",
			sql:            "SELECT 1; SELECT 2; SELECT 3",
			wantStmtCount:  3,
			wantErrorCount: 0,
		},
		{
			name:           "mix of valid and invalid",
			sql:            "SELECT 1; SELECT FROM WHERE; SELECT 3",
			wantStmtCount:  3,
			wantErrorCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := executor.ExecuteSQL(ctx, nil, tt.sql, "test_schema")

			if result.ErrorCount != tt.wantErrorCount {
				t.Errorf("Expected %d errors, got %d: %v", tt.wantErrorCount, result.ErrorCount, result.Errors)
			}
		})
	}
}

func TestExecutor_DatabaseTypes(t *testing.T) {
	tests := []struct {
		name         string
		databaseType string
		sql          string
	}{
		{
			name:         "PostgreSQL",
			databaseType: "postgresql",
			sql:          "SELECT 1",
		},
		{
			name:         "MySQL",
			databaseType: "mysql",
			sql:          "SELECT 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := dryrun.NewExecutor(tt.databaseType)
			ctx := context.Background()
			result := executor.ExecuteSQL(ctx, nil, tt.sql, "test_schema")

			if result.ErrorCount > 0 {
				t.Errorf("Expected no errors for valid SQL, got %d", result.ErrorCount)
			}
			if result.Status != models.DryRunStatusSuccess {
				t.Errorf("Expected SUCCESS status, got %s", result.Status)
			}
		})
	}
}
