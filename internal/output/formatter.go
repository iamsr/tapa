package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/iamsr/tapa/internal/ui"
	"github.com/iamsr/tapa/pkg/models"
	"gopkg.in/yaml.v3"
)

// Format outputs the analysis result in the specified format
func Format(w io.Writer, result *models.AnalysisResult, format string) error {
	switch format {
	case "table":
		return FormatTable(w, result)
	case "json":
		return FormatJSON(w, result)
	case "yaml":
		return FormatYAML(w, result)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// FormatTable outputs the result in a formatted table
func FormatTable(w io.Writer, result *models.AnalysisResult) error {
	if len(result.Migrations) == 0 {
		fmt.Fprintln(w, "No migrations analyzed")
		return nil
	}

	for _, migration := range result.Migrations {
		// Display summary card at the top
		summaryCard := ui.FormatSummaryCard(migration)
		fmt.Fprintln(w, summaryCard)
		fmt.Fprintln(w)

		// Display individual operation cards
		for i, op := range migration.Operations {
			opCard := ui.FormatOperationCard(op, i+1)
			fmt.Fprintln(w, opCard)
			fmt.Fprintln(w)
		}
	}

	// Print errors if any
	if len(result.Errors) > 0 {
		fmt.Fprintln(w, "\nErrors:")
		fmt.Fprintln(w, strings.Repeat("-", 60))
		for _, err := range result.Errors {
			fmt.Fprintf(w, "  • %v\n", err)
		}
	}

	return nil
}

// FormatJSON outputs the result as JSON
func FormatJSON(w io.Writer, result *models.AnalysisResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// FormatYAML outputs the result as YAML
func FormatYAML(w io.Writer, result *models.AnalysisResult) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(result)
}

// formatTableRow creates a table row with borders
func formatTableRow(columns []string) string {
	row := "│"
	for _, col := range columns {
		row += " " + col + " │"
	}
	return row
}

// FormatBatching outputs the batching result in the specified format
func FormatBatching(w io.Writer, result *models.BatchResult, format string) error {
	switch format {
	case "json":
		return formatBatchingJSON(w, result)
	case "yaml":
		return formatBatchingYAML(w, result)
	default:
		return formatBatchingTable(w, result)
	}
}

func formatBatchingTable(w io.Writer, result *models.BatchResult) error {
	fmt.Fprintf(w, "\nMigration Batching Strategy\n")
	fmt.Fprintf(w, "================================================================================\n\n")

	strategy := result.Strategy

	fmt.Fprintf(w, "Summary:\n")
	fmt.Fprintf(w, "  Total Operations: %d\n", strategy.TotalOperations)
	fmt.Fprintf(w, "  Total Batches: %d\n", strategy.TotalBatches)
	fmt.Fprintf(w, "  Estimated Total Time: %.2fs\n", strategy.TotalTimeSeconds)
	fmt.Fprintf(w, "  Max Risk Level: %s\n", strategy.MaxRiskLevel)
	fmt.Fprintf(w, "\n")

	for _, batch := range strategy.Batches {
		fmt.Fprintf(w, "Batch #%d (%s):\n", batch.BatchNumber, batch.RiskLevel)
		fmt.Fprintf(w, "  Operations: %d\n", len(batch.Operations))
		fmt.Fprintf(w, "  Risk Score: %d/100\n", batch.MaxRiskScore)
		fmt.Fprintf(w, "  Estimated Time: %.2fs\n", batch.TotalTimeSeconds)
		fmt.Fprintf(w, "  Parallel Execution: %v\n", batch.CanRunInParallel)

		if len(batch.Prerequisites) > 0 {
			fmt.Fprintf(w, "  Prerequisites: Batches %v\n", batch.Prerequisites)
		}

		fmt.Fprintf(w, "  Rationale: %s\n", batch.Rationale)
		fmt.Fprintf(w, "\n  Operations:\n")

		for i, op := range batch.Operations {
			fmt.Fprintf(w, "    %d. %s on %s (Risk: %d)\n", i+1, op.Type, op.TableName, op.RiskScore)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(strategy.Recommendations) > 0 {
		fmt.Fprintf(w, "Recommendations:\n")
		for _, rec := range strategy.Recommendations {
			fmt.Fprintf(w, "  %s\n", rec)
		}
	}

	return nil
}

func formatBatchingJSON(w io.Writer, result *models.BatchResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func formatBatchingYAML(w io.Writer, result *models.BatchResult) error {
	encoder := yaml.NewEncoder(w)
	return encoder.Encode(result)
}

// Helper functions

func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
