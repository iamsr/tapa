package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iamsr/tapa/internal/output"
	"github.com/iamsr/tapa/pkg/models"
)

func TestFormatDryRunResult(t *testing.T) {
	result := &models.DryRunResult{
		Status:          models.DryRunStatusFailed,
		ExecutionTimeMS: 150,
		ErrorCount:      2,
		WarningCount:    1,
		Errors: []models.ExecutionError{
			{
				ErrorType: models.ErrorTypeConstraintViolation,
				Message:   "foreign key constraint violation",
				SQL:       "ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)",
			},
			{
				ErrorType: models.ErrorTypeSyntaxError,
				Message:   "syntax error near WHERE",
				SQL:       "SELECT FROM WHERE",
			},
		},
		Warnings: []models.ExecutionWarning{
			{
				WarningType: "PERFORMANCE",
				Message:     "Large table scan detected",
			},
		},
		RolledBack: true,
	}

	var buf bytes.Buffer
	err := output.FormatDryRunResult(&buf, result)
	if err != nil {
		t.Fatalf("FormatDryRunResult failed: %v", err)
	}

	outputStr := buf.String()

	// Verify key sections are present
	if !strings.Contains(outputStr, "Dry-Run Execution") {
		t.Error("Output should contain 'Dry-Run Execution' section")
	}

	if !strings.Contains(outputStr, "FAILED") {
		t.Error("Output should show FAILED status")
	}

	if !strings.Contains(outputStr, "2 errors") {
		t.Error("Output should show error count")
	}

	if !strings.Contains(outputStr, "CONSTRAINT_VIOLATION") {
		t.Error("Output should show error types")
	}
}
