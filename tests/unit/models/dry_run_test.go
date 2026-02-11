package models_test

import (
	"strings"
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestDryRunResult_IsSuccessful(t *testing.T) {
	tests := []struct {
		name   string
		result *models.DryRunResult
		want   bool
	}{
		{
			name: "successful execution",
			result: &models.DryRunResult{
				Status:       models.DryRunStatusSuccess,
				ErrorCount:   0,
				WarningCount: 0,
			},
			want: true,
		},
		{
			name: "failed execution",
			result: &models.DryRunResult{
				Status:       models.DryRunStatusFailed,
				ErrorCount:   2,
				WarningCount: 1,
			},
			want: false,
		},
		{
			name: "success status but has errors",
			result: &models.DryRunResult{
				Status:     models.DryRunStatusSuccess,
				ErrorCount: 1,
			},
			want: false,
		},
		{
			name: "skipped status",
			result: &models.DryRunResult{
				Status:     models.DryRunStatusSkipped,
				ErrorCount: 0,
			},
			want: false,
		},
		{
			name: "timed out status",
			result: &models.DryRunResult{
				Status:     models.DryRunStatusTimedOut,
				ErrorCount: 0,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsSuccessful(); got != tt.want {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDryRunResult_HasErrors(t *testing.T) {
	tests := []struct {
		name       string
		errorCount int
		want       bool
	}{
		{
			name:       "no errors",
			errorCount: 0,
			want:       false,
		},
		{
			name:       "one error",
			errorCount: 1,
			want:       true,
		},
		{
			name:       "multiple errors",
			errorCount: 5,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &models.DryRunResult{
				ErrorCount: tt.errorCount,
			}
			if got := result.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecutionError_String(t *testing.T) {
	err := &models.ExecutionError{
		ErrorType: models.ErrorTypeConstraintViolation,
		Message:   "foreign key violation",
		SQL:       "ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)",
		Details:   "referenced table does not exist",
		Severity:  models.SeverityError,
	}

	str := err.String()
	if str == "" {
		t.Error("String() should not be empty")
	}

	// Verify format contains error type
	if !strings.Contains(str, "CONSTRAINT_VIOLATION") {
		t.Error("String() should contain error type")
	}

	// Verify format contains message
	if !strings.Contains(str, "foreign key violation") {
		t.Error("String() should contain message")
	}
}
