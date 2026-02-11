package datamigration

import (
	"context"
	"fmt"
	"strings"

	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

// Analyzer detects and analyzes data migration operations
type Analyzer struct {
	databaseType string
}

// NewAnalyzer creates a new data migration analyzer
func NewAnalyzer(databaseType string) *Analyzer {
	return &Analyzer{
		databaseType: databaseType,
	}
}

// DetectDataMigration detects if operation includes data transformation
func (a *Analyzer) DetectDataMigration(ctx context.Context, op *models.Operation, stats *db.TableStats) error {
	sqlUpper := strings.ToUpper(op.SQL)
	sqlLower := strings.ToLower(op.SQL)

	// Check for data migration patterns
	hasUpdate := strings.Contains(sqlUpper, "UPDATE ")
	hasInsertSelect := strings.Contains(sqlUpper, "INSERT") && strings.Contains(sqlUpper, "SELECT")
	hasDelete := strings.Contains(sqlUpper, "DELETE ")

	if !hasUpdate && !hasInsertSelect && !hasDelete {
		// No data migration detected
		return nil
	}

	analysis := &models.DataMigrationAnalysis{
		HasDataMigration: true,
	}

	// Determine operation type
	switch {
	case hasUpdate:
		analysis.OperationType = "UPDATE"
		a.analyzeUpdateOperation(op, stats, analysis, sqlLower)

	case hasInsertSelect:
		analysis.OperationType = "INSERT...SELECT"
		a.analyzeInsertSelectOperation(op, stats, analysis, sqlLower)

	case hasDelete:
		analysis.OperationType = "DELETE"
		a.analyzeDeleteOperation(op, stats, analysis, sqlLower)
	}

	// Estimate affected rows
	if stats != nil {
		analysis.EstimatedRows = a.estimateAffectedRows(op, stats, sqlLower)
	}

	// Calculate performance estimate
	a.calculatePerformanceEstimate(op, stats, analysis)

	// Generate batching recommendation
	if analysis.EstimatedRows > 5000 {
		a.generateBatchingRecommendation(op, analysis)
	}

	// Analyze table bloat impact (PostgreSQL MVCC)
	if a.databaseType == "postgresql" && analysis.OperationType == "UPDATE" {
		a.analyzeTableBloatImpact(op, stats, analysis)
	}

	op.DataMigrationAnalysis = analysis
	return nil
}

func (a *Analyzer) analyzeUpdateOperation(op *models.Operation, stats *db.TableStats, analysis *models.DataMigrationAnalysis, sqlLower string) {
	// Determine complexity
	if strings.Contains(sqlLower, "join") {
		analysis.Complexity = models.DataMigrationComplex
	} else if strings.Contains(sqlLower, "case") || strings.Contains(sqlLower, "coalesce") {
		analysis.Complexity = models.DataMigrationModerate
	} else {
		analysis.Complexity = models.DataMigrationSimple
	}
}

func (a *Analyzer) analyzeInsertSelectOperation(op *models.Operation, stats *db.TableStats, analysis *models.DataMigrationAnalysis, sqlLower string) {
	// INSERT...SELECT is typically complex
	if strings.Contains(sqlLower, "join") || strings.Contains(sqlLower, "group by") {
		analysis.Complexity = models.DataMigrationComplex
	} else {
		analysis.Complexity = models.DataMigrationModerate
	}
}

func (a *Analyzer) analyzeDeleteOperation(op *models.Operation, stats *db.TableStats, analysis *models.DataMigrationAnalysis, sqlLower string) {
	analysis.Complexity = models.DataMigrationBulkDelete

	// DELETE operations may free up space
	if stats != nil {
		// Estimate space impact
		// This would be more accurate with actual WHERE clause analysis
	}
}

func (a *Analyzer) estimateAffectedRows(op *models.Operation, stats *db.TableStats, sqlLower string) int64 {
	// Try to extract WHERE clause to estimate affected rows
	// This is a simplified heuristic

	if !strings.Contains(sqlLower, "where") {
		// No WHERE clause = entire table
		return stats.RowCount
	}

	// Look for common patterns
	if strings.Contains(sqlLower, "is null") {
		// Estimate 10% of rows have NULL (conservative)
		return stats.RowCount / 10
	}

	// Conservative estimate: 50% of rows
	return stats.RowCount / 2
}

func (a *Analyzer) calculatePerformanceEstimate(op *models.Operation, stats *db.TableStats, analysis *models.DataMigrationAnalysis) {
	// Base speed depends on complexity
	baseSpeed := 15000 // rows/second for simple operations

	switch analysis.Complexity {
	case models.DataMigrationSimple:
		baseSpeed = 15000
	case models.DataMigrationModerate:
		baseSpeed = 10000
	case models.DataMigrationComplex:
		baseSpeed = 5000
	case models.DataMigrationBulkDelete:
		baseSpeed = 20000
	}

	// Adjust for indexes (each index slows down writes)
	indexCount := len(stats.Indexes)
	adjustmentFactor := 1.0 - (float64(indexCount) * 0.15) // 15% slowdown per index
	if adjustmentFactor < 0.3 {
		adjustmentFactor = 0.3 // Minimum 30% of base speed
	}

	adjustedSpeed := int(float64(baseSpeed) * adjustmentFactor)

	// Calculate duration
	durationSeconds := float64(analysis.EstimatedRows) / float64(adjustedSpeed)

	// Calculate range (±20%)
	minDuration := durationSeconds * 0.8
	maxDuration := durationSeconds * 1.2

	durationRange := a.formatDurationRange(minDuration, maxDuration)

	analysis.PerformanceEstimate = &models.PerformanceEstimate{
		BaseSpeedRowsPerSecond:     baseSpeed,
		AdjustedSpeedRowsPerSecond: adjustedSpeed,
		EstimatedDurationSeconds:   durationSeconds,
		EstimatedDurationRange:     durationRange,
	}
}

func (a *Analyzer) generateBatchingRecommendation(op *models.Operation, analysis *models.DataMigrationAnalysis) {
	// Recommend batch size based on row count
	recommendedBatchSize := 10000

	if analysis.EstimatedRows > 10000000 {
		recommendedBatchSize = 5000 // Smaller batches for very large datasets
	} else if analysis.EstimatedRows < 100000 {
		recommendedBatchSize = 20000 // Larger batches for smaller datasets
	}

	totalBatches := int((analysis.EstimatedRows + int64(recommendedBatchSize) - 1) / int64(recommendedBatchSize))

	// Pause duration between batches (to allow concurrent operations)
	pauseDuration := 100 // milliseconds

	// Generate batching SQL example
	batchingSQL := a.generateBatchingSQL(op, recommendedBatchSize)

	analysis.BatchingRecommendation = &models.BatchingRecommendation{
		ShouldBatch:          true,
		RecommendedBatchSize: recommendedBatchSize,
		TotalBatches:         totalBatches,
		PauseDurationMS:      pauseDuration,
		AllowsCancellation:   true,
		BatchingSQL:          batchingSQL,
		ProgressTracking:     "Track progress by counting affected rows after each batch",
	}
}

func (a *Analyzer) generateBatchingSQL(op *models.Operation, batchSize int) string {
	// Extract UPDATE statement and add LIMIT
	sqlUpper := strings.ToUpper(op.SQL)

	if strings.Contains(sqlUpper, "UPDATE") {
		// Example batching approach for UPDATE
		return fmt.Sprintf(`-- Batched update example:
DO $$
DECLARE
  affected_rows INT;
BEGIN
  LOOP
    %s LIMIT %d;
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    EXIT WHEN affected_rows = 0;
    PERFORM pg_sleep(0.1); -- Pause between batches
  END LOOP;
END $$;`, strings.TrimRight(op.SQL, ";"), batchSize)
	}

	return "-- Batching SQL depends on operation type"
}

func (a *Analyzer) analyzeTableBloatImpact(op *models.Operation, stats *db.TableStats, analysis *models.DataMigrationAnalysis) {
	// PostgreSQL MVCC creates dead tuples on UPDATE
	estimatedBloatPercent := 15 // Conservative estimate

	if analysis.EstimatedRows > stats.RowCount/2 {
		// Updating >50% of rows = higher bloat
		estimatedBloatPercent = 30
	}

	deadTupleCount := analysis.EstimatedRows
	spaceReclaimable := int64(float64(stats.TableSizeBytes) * (float64(estimatedBloatPercent) / 100.0))

	// Estimate VACUUM duration (rough heuristic: 1 minute per 10GB)
	vacuumDuration := float64(stats.TableSizeBytes) / (10 * 1024 * 1024 * 1024) * 60 // seconds

	analysis.TableBloatImpact = &models.TableBloatImpact{
		EstimatedBloatPercent: estimatedBloatPercent,
		DeadTupleCount:        deadTupleCount,
		SpaceReclaimableBytes: spaceReclaimable,
		VacuumRequired:        true,
		VacuumDurationSeconds: vacuumDuration,
		VacuumRecommendation:  fmt.Sprintf("Run VACUUM ANALYZE %s after migration to reclaim ~%d GB", op.TableName, spaceReclaimable/(1024*1024*1024)),
	}
}

func (a *Analyzer) formatDurationRange(minSeconds, maxSeconds float64) string {
	formatDuration := func(seconds float64) string {
		if seconds < 60 {
			return fmt.Sprintf("%.0fs", seconds)
		} else if seconds < 3600 {
			return fmt.Sprintf("%.1fm", seconds/60)
		} else {
			return fmt.Sprintf("%.1fh", seconds/3600)
		}
	}

	return fmt.Sprintf("%s-%s", formatDuration(minSeconds), formatDuration(maxSeconds))
}
