package output

import (
	"fmt"
	"io"

	"github.com/iamsr/tapa/pkg/models"
)

// FormatAdvancedFeatures formats advanced feature analysis results
func FormatAdvancedFeatures(w io.Writer, op *models.Operation) error {
	if op.DiskSpaceAnalysis != nil {
		if err := formatDiskSpaceAnalysis(w, op.DiskSpaceAnalysis); err != nil {
			return err
		}
	}

	if op.RollbackAnalysis != nil {
		if err := formatRollbackAnalysis(w, op.RollbackAnalysis); err != nil {
			return err
		}
	}

	if op.DataMigrationAnalysis != nil && op.DataMigrationAnalysis.HasDataMigration {
		if err := formatDataMigrationAnalysis(w, op.DataMigrationAnalysis); err != nil {
			return err
		}
	}

	if op.DryRunResult != nil {
		if err := FormatDryRunResult(w, op.DryRunResult); err != nil {
			return err
		}
	}

	return nil
}

func formatDiskSpaceAnalysis(w io.Writer, analysis *models.DiskSpaceAnalysis) error {
	fmt.Fprintln(w, "\nDisk Space Analysis:")
	fmt.Fprintln(w, "─────────────────────────────────────")

	// Current state
	fmt.Fprintf(w, "Current State:\n")
	fmt.Fprintf(w, "  Table:   %s\n", formatBytes(analysis.CurrentState.TableSizeBytes))
	fmt.Fprintf(w, "  Indexes: %s\n", formatBytes(analysis.CurrentState.IndexSizeBytes))
	fmt.Fprintf(w, "  Total:   %s\n", formatBytes(analysis.CurrentState.TotalSizeBytes))

	// Migration requirements
	fmt.Fprintf(w, "\nMigration Requirements:\n")
	if analysis.MigrationRequirements.RequiresRewrite {
		fmt.Fprintf(w, "  Requires table rewrite: Yes\n")
		fmt.Fprintf(w, "  Temporary table: %s\n", formatBytes(analysis.MigrationRequirements.TemporaryTableBytes))
	}
	fmt.Fprintf(w, "  Peak disk usage: %s\n", formatBytes(analysis.MigrationRequirements.PeakDiskUsageBytes))

	// System check
	fmt.Fprintf(w, "\nSystem Check:\n")
	fmt.Fprintf(w, "  Available: %s\n", formatBytes(analysis.SystemCheck.AvailableBytes))
	fmt.Fprintf(w, "  Required:  %s\n", formatBytes(analysis.SystemCheck.RequiredBytes))

	status := "SUFFICIENT"
	statusColor := colorGreen
	if !analysis.SystemCheck.HasSufficientSpace {
		status = "INSUFFICIENT SPACE"
		statusColor = colorRed
		fmt.Fprintf(w, "  Shortfall: %s\n", formatBytes(analysis.SystemCheck.ShortfallBytes))
	}
	fmt.Fprintf(w, "  Status:    %s%s%s\n", statusColor, status, colorReset)

	return nil
}

func formatRollbackAnalysis(w io.Writer, analysis *models.RollbackAnalysis) error {
	fmt.Fprintln(w, "\nRollback Analysis:")
	fmt.Fprintln(w, "─────────────────────────────────────")

	// Category with color
	categoryColor := colorGreen
	switch analysis.Category {
	case models.ReversibilitySafe:
		categoryColor = colorGreen
	case models.ReversibilityConditional:
		categoryColor = colorYellow
	case models.ReversibilityDataLoss, models.ReversibilityIrreversible:
		categoryColor = colorRed
	}

	fmt.Fprintf(w, "Category: %s%s%s\n", categoryColor, analysis.Category, colorReset)
	fmt.Fprintf(w, "Reversibility Score: %d/100\n", analysis.ReversibilityScore)
	fmt.Fprintf(w, "Reversible: %v\n", analysis.IsReversible)
	fmt.Fprintf(w, "Reason: %s\n", analysis.Reason)

	// Auto-rollback SQL
	if analysis.AutoRollbackSQL != "" {
		fmt.Fprintf(w, "\nAuto-generated Rollback:\n")
		fmt.Fprintf(w, "  %s\n", analysis.AutoRollbackSQL)
	}

	// Recovery strategy
	if analysis.RecoveryStrategy != nil {
		fmt.Fprintf(w, "\nRecovery Strategy:\n")
		fmt.Fprintf(w, "  Method: %s\n", analysis.RecoveryStrategy.Method)
		if analysis.RecoveryStrategy.EstimatedRTO != "" {
			fmt.Fprintf(w, "  Estimated RTO: %s\n", analysis.RecoveryStrategy.EstimatedRTO)
		}
		if len(analysis.RecoveryStrategy.Steps) > 0 {
			fmt.Fprintf(w, "  Steps:\n")
			for i, step := range analysis.RecoveryStrategy.Steps {
				fmt.Fprintf(w, "    %d. %s\n", i+1, step)
			}
		}
	}

	return nil
}

func formatDataMigrationAnalysis(w io.Writer, analysis *models.DataMigrationAnalysis) error {
	fmt.Fprintln(w, "\nData Migration Analysis:")
	fmt.Fprintln(w, "─────────────────────────────────────")

	fmt.Fprintf(w, "Operation: %s\n", analysis.OperationType)
	fmt.Fprintf(w, "Complexity: %s\n", analysis.Complexity)
	fmt.Fprintf(w, "Estimated rows: %s\n", formatNumber(analysis.EstimatedRows))

	// Performance estimate
	if analysis.PerformanceEstimate != nil {
		fmt.Fprintf(w, "\nPerformance Estimate:\n")
		fmt.Fprintf(w, "  Base speed: %s rows/second\n", formatNumber(int64(analysis.PerformanceEstimate.BaseSpeedRowsPerSecond)))
		fmt.Fprintf(w, "  Adjusted speed: %s rows/second\n", formatNumber(int64(analysis.PerformanceEstimate.AdjustedSpeedRowsPerSecond)))
		fmt.Fprintf(w, "  Estimated duration: %s\n", analysis.PerformanceEstimate.EstimatedDurationRange)
	}

	// Batching recommendation
	if analysis.BatchingRecommendation != nil && analysis.BatchingRecommendation.ShouldBatch {
		fmt.Fprintf(w, "\nBatching Recommendation:\n")
		fmt.Fprintf(w, "  Should batch: Yes\n")
		fmt.Fprintf(w, "  Recommended batch size: %s\n", formatNumber(int64(analysis.BatchingRecommendation.RecommendedBatchSize)))
		fmt.Fprintf(w, "  Total batches: %d\n", analysis.BatchingRecommendation.TotalBatches)
		fmt.Fprintf(w, "  Allows cancellation: %v\n", analysis.BatchingRecommendation.AllowsCancellation)
	}

	// Table bloat impact
	if analysis.TableBloatImpact != nil && analysis.TableBloatImpact.VacuumRequired {
		fmt.Fprintf(w, "\nTable Bloat Impact:\n")
		fmt.Fprintf(w, "  Estimated bloat: %d%%\n", analysis.TableBloatImpact.EstimatedBloatPercent)
		fmt.Fprintf(w, "  Space reclaimable: %s\n", formatBytes(analysis.TableBloatImpact.SpaceReclaimableBytes))
		fmt.Fprintf(w, "  Vacuum required: Yes\n")
		fmt.Fprintf(w, "  Recommendation: %s\n", analysis.TableBloatImpact.VacuumRecommendation)
	}

	return nil
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.2fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
