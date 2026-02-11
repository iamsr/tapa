package models_test

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsSuccessful(); got != tt.want {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.want)
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
	}

	str := err.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
}
