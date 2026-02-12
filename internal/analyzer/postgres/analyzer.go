package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/iamsr/tapa/internal/analyzer/alternatives"
	"github.com/iamsr/tapa/internal/analyzer/batcher"
	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/internal/analyzer/datamigration"
	"github.com/iamsr/tapa/internal/analyzer/dependencies"
	"github.com/iamsr/tapa/internal/analyzer/diskspace"
	"github.com/iamsr/tapa/internal/analyzer/estimator"
	"github.com/iamsr/tapa/internal/analyzer/rollback"
	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

// Analyzer analyzes PostgreSQL operations for production impact
type Analyzer struct {
	introspector       db.Introspector
	diskThroughputMBps int
	rewriteFactor      float64
	comprehensive      bool

	// Phase 2 modules
	dependencyAnalyzer   dependencies.DependencyAnalyzer
	timeEstimator        estimator.TimeEstimator
	alternativeGenerator alternatives.AlternativeGenerator
	batcher              batcher.MigrationBatcher

	// Advanced analyzers
	concurrencyAnalyzer *concurrency.Analyzer
}

// NewAnalyzer creates a new PostgreSQL analyzer
func NewAnalyzer(introspector db.Introspector, diskThroughputMBps int, rewriteFactor float64, comprehensive bool) *Analyzer {
	analyzer := &Analyzer{
		introspector:       introspector,
		diskThroughputMBps: diskThroughputMBps,
		rewriteFactor:      rewriteFactor,
		comprehensive:      comprehensive,
	}

	// Initialize Phase 2 modules (errors are ignored for optional features)
	analyzer.dependencyAnalyzer, _ = dependencies.GetDependencyAnalyzer("postgresql", introspector)
	analyzer.timeEstimator, _ = estimator.GetTimeEstimator("postgresql", introspector, diskThroughputMBps, rewriteFactor)
	analyzer.alternativeGenerator, _ = alternatives.GetAlternativeGenerator("postgresql")
	analyzer.batcher, _ = batcher.GetMigrationBatcher("postgresql")

	return analyzer
}

// SetConcurrencyAnalyzer sets the concurrency analyzer for advanced concurrency analysis
func (a *Analyzer) SetConcurrencyAnalyzer(concurrencyAnalyzer *concurrency.Analyzer) {
	a.concurrencyAnalyzer = concurrencyAnalyzer
}

// Analyze enriches an operation with lock detection, risk scoring, and recommendations
func (a *Analyzer) Analyze(ctx context.Context, op *models.Operation) error {
	// Step 1: Detect lock type and requirements
	a.detectLockType(op)

	// Step 2: Get table stats (if needed)
	var stats *db.TableStats
	var err error
	if op.TableName != "" && op.Type != models.OperationTypeCreateTable && a.introspector != nil {
		stats, err = a.introspector.GetTableStats(ctx, op.TableName)
		if err != nil {
			// If we can't get stats, use conservative estimates
			stats = &db.TableStats{
				TableName:      op.TableName,
				RowCount:       1000000,                 // Assume 1M rows
				TableSizeBytes: 10 * 1024 * 1024 * 1024, // Assume 10 GB
			}
		}
	} else if op.TableName != "" && op.Type != models.OperationTypeCreateTable {
		// No introspector - use conservative estimates
		stats = &db.TableStats{
			TableName:      op.TableName,
			RowCount:       1000000,                 // Assume 1M rows
			TableSizeBytes: 10 * 1024 * 1024 * 1024, // Assume 10 GB
		}
	}

	// Step 3: Store table stats on operation
	if stats != nil {
		op.RowCount = stats.RowCount
		op.TableSizeBytes = stats.TableSizeBytes
	}

	// Step 4: Estimate duration
	a.estimateDuration(op, stats)

	// Step 5: Calculate risk score
	a.calculateRiskScore(op, stats)

	// Step 6: Determine backward compatibility
	a.setBackwardCompatibility(op)

	// Step 7: Generate recommendations
	a.generateRecommendations(op, stats)

	// Step 8: Run advanced analyzers in comprehensive mode
	if a.comprehensive {
		if err := a.runAdvancedAnalyzers(ctx, op, stats); err != nil {
			// Non-fatal: advanced analysis failures don't stop main analysis
			// In production, this would use proper logging
		}
	}

	return nil
}

// detectLockType determines the lock type based on operation type and SQL
func (a *Analyzer) detectLockType(op *models.Operation) {
	sqlUpper := strings.ToUpper(op.SQL)
	sqlLower := strings.ToLower(op.SQL)

	switch op.Type {
	case models.OperationTypeAddColumn:
		// ADD COLUMN requires ACCESS EXCLUSIVE lock
		op.LockType = models.LockTypeAccessExclusive
		op.LockDurationMS = 100

		// Check if it has DEFAULT value (requires rewrite in older PG, instant in PG 11+ for constant defaults)
		if strings.Contains(sqlLower, "default") {
			// Check for volatile functions
			volatileFuncs := []string{"now()", "random()", "uuid_generate", "gen_random_uuid()"}
			for _, fn := range volatileFuncs {
				if strings.Contains(sqlLower, fn) {
					op.RequiresRewrite = true
					break
				}
			}

			// In PostgreSQL < 11, all DEFAULTs require rewrite
			// For this analyzer, assume we need to be safe and mark as rewrite
			// unless it's clearly NULL or no default
			if !op.RequiresRewrite && strings.Contains(sqlLower, "default") {
				// Constant default still requires rewrite (to be conservative)
				op.RequiresRewrite = true
			}
		}

	case models.OperationTypeDropColumn:
		// DROP COLUMN requires ACCESS EXCLUSIVE lock
		// In PG 11+, it's instant (metadata only)
		op.LockType = models.LockTypeAccessExclusive
		op.LockDurationMS = 100
		op.RequiresRewrite = false

	case models.OperationTypeAlterColumn:
		// ALTER COLUMN typically requires table rewrite
		op.LockType = models.LockTypeAccessExclusive
		op.RequiresRewrite = true
		op.LockDurationMS = 1000

		// Some type changes don't require rewrite
		if strings.Contains(sqlLower, "varchar") {
			// Increasing VARCHAR length doesn't require rewrite
			if !strings.Contains(sqlLower, "using") {
				// Check if it's just increasing length (simple heuristic)
				if !strings.Contains(sqlLower, "type integer") &&
					!strings.Contains(sqlLower, "type numeric") &&
					!strings.Contains(sqlLower, "type bigint") {
					// Might be VARCHAR to VARCHAR with larger length
					// Still mark as rewrite to be safe
				}
			}
		}

	case models.OperationTypeCreateIndex:
		if strings.Contains(sqlUpper, "CONCURRENTLY") {
			// CONCURRENTLY uses SHARE UPDATE EXCLUSIVE briefly
			op.LockType = models.LockTypeShareUpdateExclusive
			op.LockDurationMS = 50
		} else {
			// Without CONCURRENTLY: SHARE lock blocks writes
			op.LockType = models.LockTypeShare
			op.LockDurationMS = 100
		}

	case models.OperationTypeDropIndex:
		if strings.Contains(sqlUpper, "CONCURRENTLY") {
			op.LockType = models.LockTypeShareUpdateExclusive
			op.LockDurationMS = 50
		} else {
			op.LockType = models.LockTypeAccessExclusive
			op.LockDurationMS = 100
		}

	case models.OperationTypeCreateTable:
		// CREATE TABLE doesn't lock existing tables
		op.LockType = models.LockTypeNone
		op.LockDurationMS = 0

	case models.OperationTypeDropTable:
		// DROP TABLE requires ACCESS EXCLUSIVE on that table
		op.LockType = models.LockTypeAccessExclusive
		op.LockDurationMS = 50

	default:
		// Unknown operation - assume worst case
		op.LockType = models.LockTypeAccessExclusive
		op.LockDurationMS = 100
	}
}

// estimateDuration calculates estimated duration based on table size and operation
func (a *Analyzer) estimateDuration(op *models.Operation, stats *db.TableStats) {
	if op.RequiresRewrite && stats != nil {
		// Table rewrite: Time = (TableSize / Throughput) * RewriteFactor
		tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
		baseTimeSeconds := tableSizeMB / float64(a.diskThroughputMBps)
		estimatedTime := baseTimeSeconds * a.rewriteFactor

		op.EstimatedTimeSeconds = estimatedTime
		op.LockDurationMS = int64(estimatedTime * 1000)
	} else if op.Type == models.OperationTypeCreateIndex && stats != nil {
		// Index build time (rough estimate based on table size)
		tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
		// Index builds are typically slower than raw I/O
		indexBuildTime := tableSizeMB / float64(a.diskThroughputMBps) * 1.5

		op.EstimatedTimeSeconds = indexBuildTime
		// For CONCURRENTLY, lock duration is minimal
		if op.LockType == models.LockTypeShareUpdateExclusive {
			op.LockDurationMS = 50
		} else {
			// SHARE lock held during entire index build
			op.LockDurationMS = int64(indexBuildTime * 1000)
		}
	} else {
		// Fast metadata operations
		op.EstimatedTimeSeconds = float64(op.LockDurationMS) / 1000.0
	}
}

// calculateRiskScore computes risk score (0-100) based on lock type, table size, and duration
func (a *Analyzer) calculateRiskScore(op *models.Operation, stats *db.TableStats) {
	// Risk score formula from spec:
	// riskScore = baseLockScore + tableSizeScore + estimatedDurationScore
	// - baseLockScore: ACCESS EXCLUSIVE=40, SHARE=20, others=10
	// - tableSizeScore: (tableSize / 10GB) * 30 (max 30)
	// - durationScore: (estimatedMinutes / 60) * 30 (max 30)

	baseLockScore := 0
	switch op.LockType {
	case models.LockTypeAccessExclusive:
		baseLockScore = 40
	case models.LockTypeShare:
		baseLockScore = 20
	case models.LockTypeShareUpdateExclusive:
		baseLockScore = 10
	case models.LockTypeNone:
		baseLockScore = 0
	default:
		baseLockScore = 10
	}

	tableSizeScore := 0
	if stats != nil && stats.TableSizeBytes > 0 {
		tableSizeGB := float64(stats.TableSizeBytes) / (1024 * 1024 * 1024)
		tableSizeScore = int(math.Min((tableSizeGB/10.0)*30, 30))
	}

	durationScore := 0
	estimatedMinutes := op.EstimatedTimeSeconds / 60.0
	durationScore = int(math.Min((estimatedMinutes/60.0)*30, 30))

	op.RiskScore = baseLockScore + tableSizeScore + durationScore

	// Ensure risk score is in valid range
	if op.RiskScore > 100 {
		op.RiskScore = 100
	}
	if op.RiskScore < 0 {
		op.RiskScore = 0
	}
}

// setBackwardCompatibility determines if operation is backward compatible
func (a *Analyzer) setBackwardCompatibility(op *models.Operation) {
	switch op.Type {
	case models.OperationTypeDropColumn, models.OperationTypeDropTable, models.OperationTypeDropIndex:
		// Dropping things breaks backward compatibility
		op.BackwardCompatible = false

	case models.OperationTypeAlterColumn:
		// Type changes break compatibility
		op.BackwardCompatible = false

	case models.OperationTypeAddColumn:
		// Adding nullable column is backward compatible
		// Adding column with DEFAULT might have issues with old code
		sqlLower := strings.ToLower(op.SQL)
		if strings.Contains(sqlLower, "not null") && !strings.Contains(sqlLower, "default") {
			// NOT NULL without DEFAULT breaks compatibility
			op.BackwardCompatible = false
		} else {
			op.BackwardCompatible = true
		}

	case models.OperationTypeCreateIndex:
		// Adding index is backward compatible
		op.BackwardCompatible = true

	case models.OperationTypeCreateTable:
		// Creating new table is backward compatible
		op.BackwardCompatible = true

	default:
		// Unknown - assume not compatible to be safe
		op.BackwardCompatible = false
	}
}

// generateRecommendations provides actionable recommendations based on the operation
func (a *Analyzer) generateRecommendations(op *models.Operation, stats *db.TableStats) {
	op.Recommendations = []string{}

	switch op.Type {
	case models.OperationTypeAddColumn:
		if op.RequiresRewrite {
			op.Recommendations = append(op.Recommendations,
				"Add column without DEFAULT first, then set DEFAULT separately to avoid table rewrite")
		}
		sqlLower := strings.ToLower(op.SQL)
		if strings.Contains(sqlLower, "not null") && !strings.Contains(sqlLower, "default") {
			op.Recommendations = append(op.Recommendations,
				"Consider adding column as nullable first, then add NOT NULL constraint after backfilling")
		}

	case models.OperationTypeCreateIndex:
		sqlUpper := strings.ToUpper(op.SQL)
		if !strings.Contains(sqlUpper, "CONCURRENTLY") {
			if stats != nil && stats.TableSizeBytes > 1*1024*1024*1024 { // > 1 GB
				op.Recommendations = append(op.Recommendations,
					"Use CREATE INDEX CONCURRENTLY to avoid blocking writes on large table")
			}
		}

	case models.OperationTypeAlterColumn:
		if op.RequiresRewrite {
			op.Recommendations = append(op.Recommendations,
				"Type changes require full table rewrite - consider creating new column, migrating data, then dropping old column")
		}

	case models.OperationTypeDropColumn:
		op.Recommendations = append(op.Recommendations,
			"Ensure application code no longer references this column before dropping")

	case models.OperationTypeDropTable:
		op.Recommendations = append(op.Recommendations,
			"Ensure no application code or other tables reference this table")
	}

	// Add risk-based recommendations
	if op.RiskScore >= 76 {
		op.Recommendations = append(op.Recommendations,
			"CRITICAL RISK: Consider performing this operation during maintenance window")
	} else if op.RiskScore >= 51 {
		op.Recommendations = append(op.Recommendations,
			"HIGH RISK: Test thoroughly in staging and perform during low-traffic period")
	}

	// Add duration-based recommendations
	if op.EstimatedTimeSeconds > 300 { // > 5 minutes
		op.Recommendations = append(op.Recommendations,
			fmt.Sprintf("Estimated duration: %.1f minutes - plan for extended lock time", op.EstimatedTimeSeconds/60.0))
	}
}

// AnalysisOptions controls which Phase 2 features are enabled
type AnalysisOptions struct {
	IncludeDependencies  bool
	IncludeTimeBreakdown bool
	IncludeAlternatives  bool
}

// DefaultAnalysisOptions returns options with all features enabled
func DefaultAnalysisOptions() AnalysisOptions {
	return AnalysisOptions{
		IncludeDependencies:  true,
		IncludeTimeBreakdown: true,
		IncludeAlternatives:  true,
	}
}

// AnalyzeWithEnhancements performs full analysis including Phase 2 enhancements
func (a *Analyzer) AnalyzeWithEnhancements(ctx context.Context, op *models.Operation, opts AnalysisOptions) error {
	// Step 1: Run base analysis (Phase 1)
	if err := a.Analyze(ctx, op); err != nil {
		return fmt.Errorf("base analysis failed: %w", err)
	}

	// Step 2: Enrich with Phase 2 features
	if opts.IncludeDependencies && a.dependencyAnalyzer != nil {
		deps, err := a.dependencyAnalyzer.FindDependencies(ctx, op)
		if err != nil {
			// Log error but don't fail - Phase 2 features are optional
			// In production, this would use a logger
			fmt.Printf("Warning: failed to find dependencies: %v\n", err)
		} else {
			op.Dependencies = deps
		}
	}

	if opts.IncludeTimeBreakdown && a.timeEstimator != nil {
		timeBreakdown, err := a.timeEstimator.EstimateTime(ctx, op)
		if err != nil {
			fmt.Printf("Warning: failed to estimate time breakdown: %v\n", err)
		} else {
			op.TimeBreakdown = timeBreakdown
		}
	}

	if opts.IncludeAlternatives && a.alternativeGenerator != nil {
		alternatives, err := a.alternativeGenerator.GenerateAlternatives(ctx, op)
		if err != nil {
			fmt.Printf("Warning: failed to generate alternatives: %v\n", err)
		} else {
			op.Alternatives = alternatives
		}
	}

	return nil
}

// BatchOperations generates a batching strategy for multiple operations
func (a *Analyzer) BatchOperations(ops []*models.Operation) (*models.BatchingStrategy, error) {
	if a.batcher == nil {
		return nil, fmt.Errorf("batcher not initialized")
	}

	return a.batcher.GenerateBatches(ops)
}

// runAdvancedAnalyzers runs all advanced analyzers (disk space, rollback, data migration, concurrency)
func (a *Analyzer) runAdvancedAnalyzers(ctx context.Context, op *models.Operation, stats *db.TableStats) error {
	// Disk space analyzer
	diskAnalyzer := diskspace.NewAnalyzer("postgresql", a.diskThroughputMBps)
	if err := diskAnalyzer.AnalyzeDiskSpace(ctx, op, stats); err != nil {
		return fmt.Errorf("disk space analysis failed: %w", err)
	}

	// Rollback analyzer
	rollbackAnalyzer := rollback.NewAnalyzer("postgresql")
	if err := rollbackAnalyzer.AnalyzeRollback(ctx, op); err != nil {
		return fmt.Errorf("rollback analysis failed: %w", err)
	}

	// Data migration detector
	dataMigrationAnalyzer := datamigration.NewAnalyzer("postgresql")
	if err := dataMigrationAnalyzer.DetectDataMigration(ctx, op, stats); err != nil {
		return fmt.Errorf("data migration detection failed: %w", err)
	}

	// Concurrency analyzer (if available)
	if a.concurrencyAnalyzer != nil {
		if err := a.concurrencyAnalyzer.AnalyzeOperation(ctx, op); err != nil {
			// Non-fatal: concurrency analysis failures don't stop main analysis
			return fmt.Errorf("concurrency analysis failed: %w", err)
		}
	}

	return nil
}

// getDefaultStats returns default stats when introspector is not available
func (a *Analyzer) getDefaultStats(tableName string) *db.TableStats {
	return &db.TableStats{
		TableName:      tableName,
		RowCount:       1000000,
		TableSizeBytes: 10 * 1024 * 1024 * 1024, // 10GB
		IndexSizeBytes: 2 * 1024 * 1024 * 1024,  // 2GB
	}
}
