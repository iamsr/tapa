package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
		fmt.Fprintf(w, "\nMigration Analysis: %s\n", migration.FilePath)
		fmt.Fprintln(w, strings.Repeat("=", 80))

		if len(migration.Operations) == 0 {
			fmt.Fprintln(w, "No operations detected")
			continue
		}

		fmt.Fprintln(w, "\nOperations Detected:")
		printOperationsTable(w, migration.Operations)
	}

	// Print errors if any
	if len(result.Errors) > 0 {
		fmt.Fprintln(w, "\nErrors:")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for _, err := range result.Errors {
			fmt.Fprintf(w, "  • %v\n", err)
		}
	}

	return nil
}

// printOperationsTable prints operations in a table format
func printOperationsTable(w io.Writer, operations []*models.Operation) {
	// Table header
	fmt.Fprintln(w, "┌────────────────────┬─────────────────┬──────────────────────────────────────┐")
	fmt.Fprintln(w, "│ OPERATION          │ TABLE           │ DETAILS                              │")
	fmt.Fprintln(w, "├────────────────────┼─────────────────┼──────────────────────────────────────┤")

	// Table rows
	for _, op := range operations {
		details := formatOperationDetails(op)
		opType := padRight(string(op.Type), 18)
		table := padRight(op.TableName, 15)
		detail := padRight(details, 36)
		fmt.Fprintf(w, "│ %s │ %s │ %s │\n", opType, table, detail)
	}

	// Table footer
	fmt.Fprintln(w, "└────────────────────┴─────────────────┴──────────────────────────────────────┘")

	// Additional details
	fmt.Fprintln(w, "\nOperation Details:")
	for i, op := range operations {
		fmt.Fprintf(w, "\n%d. %s on table '%s'\n", i+1, op.Type, op.TableName)
		fmt.Fprintf(w, "   SQL: %s\n", truncate(op.SQL, 70))

		// Apply color to lock type
		lockTypeStr := Colorize(string(op.LockType), LockTypeColor(op.LockType))
		fmt.Fprintf(w, "   Lock Type: %s (Duration: %dms)\n", lockTypeStr, op.LockDurationMS)

		// Apply color to risk score
		riskScoreStr := Colorize(fmt.Sprintf("%d/100", op.RiskScore), RiskColor(op.RiskScore))
		fmt.Fprintf(w, "   Risk Score: %s (%s)\n", riskScoreStr, op.RiskLevel())

		fmt.Fprintf(w, "   Estimated Time: %.2fs\n", op.EstimatedTimeSeconds)
		fmt.Fprintf(w, "   Requires Rewrite: %v\n", op.RequiresRewrite)
		fmt.Fprintf(w, "   Backward Compatible: %v\n", op.BackwardCompatible)

		if len(op.Recommendations) > 0 {
			fmt.Fprintln(w, "   Recommendations:")
			for _, rec := range op.Recommendations {
				fmt.Fprintf(w, "     • %s\n", rec)
			}
		}
	}
}

// formatOperationDetails creates a short summary for the table
func formatOperationDetails(op *models.Operation) string {
	details := []string{}

	if op.RequiresRewrite {
		details = append(details, "Rewrite")
	}

	if op.LockType == models.LockTypeAccessExclusive {
		details = append(details, "Exclusive Lock")
	}

	if op.IsHighRisk() {
		details = append(details, fmt.Sprintf("Risk:%d", op.RiskScore))
	}

	if len(details) == 0 {
		return "Low impact"
	}

	return strings.Join(details, ", ")
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
