package output

import (
	"fmt"
	"io"

	"github.com/iamsr/tapa/pkg/models"
)

// FormatDryRunResult formats dry-run execution results
func FormatDryRunResult(w io.Writer, result *models.DryRunResult) error {
	if result == nil {
		return nil
	}

	fmt.Fprintln(w, "\nDry-Run Execution:")
	fmt.Fprintln(w, "─────────────────────────────────────")

	// Status with color
	statusColor := colorGreen
	switch result.Status {
	case models.DryRunStatusSuccess:
		statusColor = colorGreen
	case models.DryRunStatusFailed:
		statusColor = colorRed
	case models.DryRunStatusSkipped:
		statusColor = colorYellow
	case models.DryRunStatusTimedOut:
		statusColor = colorRed
	}

	fmt.Fprintf(w, "Status: %s%s%s\n", statusColor, result.Status, colorReset)
	fmt.Fprintf(w, "Execution time: %d ms\n", result.ExecutionTimeMS)
	fmt.Fprintf(w, "Errors: %d\n", result.ErrorCount)
	fmt.Fprintf(w, "Warnings: %d\n", result.WarningCount)

	if result.TempSchemaName != "" {
		fmt.Fprintf(w, "Temp schema: %s\n", result.TempSchemaName)
	}

	fmt.Fprintf(w, "Rolled back: %v\n", result.RolledBack)

	// Display errors
	if len(result.Errors) > 0 {
		fmt.Fprintln(w, "\nErrors:")
		for i, err := range result.Errors {
			fmt.Fprintf(w, "\n  %d. [%s] %s\n", i+1, err.ErrorType, err.Message)
			if err.SQL != "" {
				// Truncate long SQL
				sql := err.SQL
				if len(sql) > 80 {
					sql = sql[:77] + "..."
				}
				fmt.Fprintf(w, "     SQL: %s\n", sql)
			}
			if err.Details != "" {
				fmt.Fprintf(w, "     Details: %s\n", err.Details)
			}
		}
	}

	// Display warnings
	if len(result.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for i, warning := range result.Warnings {
			fmt.Fprintf(w, "  %d. [%s] %s\n", i+1, warning.WarningType, warning.Message)
			if warning.Suggestion != "" {
				fmt.Fprintf(w, "     Suggestion: %s\n", warning.Suggestion)
			}
		}
	}

	// Summary message
	if result.Status == models.DryRunStatusSuccess {
		fmt.Fprintf(w, "\n%s✓ Migration executed successfully in dry-run mode%s\n", colorGreen, colorReset)
	} else if result.Status == models.DryRunStatusFailed {
		fmt.Fprintf(w, "\n%s✗ Migration would fail with %d errors%s\n", colorRed, result.ErrorCount, colorReset)
	}

	return nil
}
