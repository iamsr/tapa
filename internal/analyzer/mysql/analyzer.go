package mysql

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/yourusername/dma/internal/db"
	"github.com/yourusername/dma/pkg/models"
)

// Analyzer analyzes MySQL operations for production impact
type Analyzer struct {
	introspector       db.Introspector
	diskThroughputMBps int
	rewriteFactor      float64
}

// NewAnalyzer creates a new MySQL analyzer
func NewAnalyzer(introspector db.Introspector, diskThroughputMBps int, rewriteFactor float64) *Analyzer {
	return &Analyzer{
		introspector:       introspector,
		diskThroughputMBps: diskThroughputMBps,
		rewriteFactor:      rewriteFactor,
	}
}

// Analyze enriches an operation with lock detection, risk scoring, and recommendations
func (a *Analyzer) Analyze(ctx context.Context, op *models.Operation) error {
	// Step 1: Detect ALGORITHM and LOCK from SQL
	algorithm := a.detectAlgorithm(op.SQL)
	lockClause := a.detectLock(op.SQL)

	// Step 2: Route to operation-specific analyzer
	switch op.Type {
	case models.OperationTypeAddColumn:
		a.analyzeAddColumn(op, algorithm, lockClause)
	case models.OperationTypeDropColumn:
		a.analyzeDropColumn(op, algorithm, lockClause)
	case models.OperationTypeAlterColumn:
		a.analyzeAlterColumn(op, algorithm, lockClause)
	case models.OperationTypeCreateIndex:
		a.analyzeCreateIndex(op, algorithm, lockClause)
	case models.OperationTypeDropIndex:
		a.analyzeDropIndex(op, algorithm, lockClause)
	case models.OperationTypeCreateTable:
		a.analyzeCreateTable(op)
	case models.OperationTypeDropTable:
		a.analyzeDropTable(op)
	default:
		// Unknown operation - assume worst case
		op.LockType = models.LockTypeExclusive
		op.RequiresRewrite = true
		op.LockDurationMS = 100
	}

	// Step 3: Get table stats (if needed)
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

	// Step 4: Estimate duration
	a.estimateDuration(op, stats)

	// Step 5: Calculate risk score
	a.calculateRiskScore(op, stats)

	// Step 6: Set backward compatibility
	a.setBackwardCompatibility(op)

	// Step 7: Generate recommendations
	a.generateRecommendations(op, stats)

	return nil
}

// detectAlgorithm extracts ALGORITHM clause from SQL (case-insensitive)
func (a *Analyzer) detectAlgorithm(sql string) string {
	re := regexp.MustCompile(`(?i)ALGORITHM\s*=\s*(DEFAULT|INPLACE|COPY|INSTANT)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return strings.ToUpper(matches[1])
	}
	return "DEFAULT"
}

// detectLock extracts LOCK clause from SQL (case-insensitive)
func (a *Analyzer) detectLock(sql string) string {
	re := regexp.MustCompile(`(?i)LOCK\s*=\s*(DEFAULT|NONE|SHARED|EXCLUSIVE)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return strings.ToUpper(matches[1])
	}
	return "DEFAULT"
}

// analyzeAddColumn analyzes ADD COLUMN operations
func (a *Analyzer) analyzeAddColumn(op *models.Operation, algorithm, lockClause string) {
	sqlLower := strings.ToLower(op.SQL)

	// Check if it has DEFAULT value
	hasDefault := strings.Contains(sqlLower, "default")

	if hasDefault {
		// ADD COLUMN with DEFAULT requires COPY in MySQL 5.7
		// In MySQL 8.0+ it can use INSTANT, but we're conservative
		op.LockType = models.LockTypeExclusive
		op.RequiresRewrite = true
		op.LockDurationMS = 1000
	} else {
		// ADD COLUMN without DEFAULT uses INPLACE, no lock needed
		if algorithm == "COPY" {
			op.LockType = models.LockTypeExclusive
			op.RequiresRewrite = true
			op.LockDurationMS = 1000
		} else {
			// INPLACE or INSTANT
			op.LockType = models.LockTypeNone
			op.RequiresRewrite = false
			op.LockDurationMS = 50
		}
	}

	// Override with explicit LOCK clause if present
	if lockClause != "DEFAULT" {
		op.LockType = a.convertLockType(lockClause)
	}
}

// analyzeDropColumn analyzes DROP COLUMN operations
func (a *Analyzer) analyzeDropColumn(op *models.Operation, algorithm, lockClause string) {
	// DROP COLUMN uses INPLACE by default
	op.LockType = models.LockTypeNone
	op.RequiresRewrite = false
	op.LockDurationMS = 50

	// COPY algorithm forces rewrite
	if algorithm == "COPY" {
		op.LockType = models.LockTypeExclusive
		op.RequiresRewrite = true
		op.LockDurationMS = 1000
	}

	// Override with explicit LOCK clause if present
	if lockClause != "DEFAULT" {
		op.LockType = a.convertLockType(lockClause)
	}
}

// analyzeAlterColumn analyzes ALTER COLUMN operations
func (a *Analyzer) analyzeAlterColumn(op *models.Operation, algorithm, lockClause string) {
	// ALTER COLUMN typically requires COPY algorithm
	op.LockType = models.LockTypeExclusive
	op.RequiresRewrite = true
	op.LockDurationMS = 1000

	// Override with explicit LOCK clause if present
	if lockClause != "DEFAULT" {
		op.LockType = a.convertLockType(lockClause)
	}
}

// analyzeCreateIndex analyzes CREATE INDEX operations
func (a *Analyzer) analyzeCreateIndex(op *models.Operation, algorithm, lockClause string) {
	// CREATE INDEX uses INPLACE by default in MySQL 5.6+
	op.LockType = models.LockTypeNone
	op.RequiresRewrite = false
	op.LockDurationMS = 100

	// COPY algorithm forces exclusive lock
	if algorithm == "COPY" {
		op.LockType = models.LockTypeExclusive
		op.RequiresRewrite = true
	}

	// Override with explicit LOCK clause if present
	if lockClause != "DEFAULT" {
		op.LockType = a.convertLockType(lockClause)
	}
}

// analyzeDropIndex analyzes DROP INDEX operations
func (a *Analyzer) analyzeDropIndex(op *models.Operation, algorithm, lockClause string) {
	// DROP INDEX uses INPLACE by default
	op.LockType = models.LockTypeNone
	op.RequiresRewrite = false
	op.LockDurationMS = 50

	// COPY algorithm forces exclusive lock
	if algorithm == "COPY" {
		op.LockType = models.LockTypeExclusive
		op.RequiresRewrite = true
	}

	// Override with explicit LOCK clause if present
	if lockClause != "DEFAULT" {
		op.LockType = a.convertLockType(lockClause)
	}
}

// analyzeCreateTable analyzes CREATE TABLE operations
func (a *Analyzer) analyzeCreateTable(op *models.Operation) {
	// CREATE TABLE doesn't lock existing tables
	op.LockType = models.LockTypeNone
	op.RequiresRewrite = false
	op.LockDurationMS = 0
}

// analyzeDropTable analyzes DROP TABLE operations
func (a *Analyzer) analyzeDropTable(op *models.Operation) {
	// DROP TABLE requires exclusive lock on that table
	op.LockType = models.LockTypeExclusive
	op.RequiresRewrite = false
	op.LockDurationMS = 50
}

// convertLockType converts MySQL lock clause to LockType
func (a *Analyzer) convertLockType(lockClause string) models.LockType {
	switch lockClause {
	case "NONE":
		return models.LockTypeNone
	case "SHARED":
		return models.LockTypeShare
	case "EXCLUSIVE":
		return models.LockTypeExclusive
	default:
		return models.LockTypeNone
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
		// For online index creation (INPLACE), lock duration is minimal
		if op.LockType == models.LockTypeNone {
			op.LockDurationMS = 100
		} else {
			// Exclusive lock held during entire index build
			op.LockDurationMS = int64(indexBuildTime * 1000)
		}
	} else {
		// Fast metadata operations
		op.EstimatedTimeSeconds = float64(op.LockDurationMS) / 1000.0
	}
}

// calculateRiskScore computes risk score (0-100) based on lock type, rewrite, duration, and compatibility
func (a *Analyzer) calculateRiskScore(op *models.Operation, stats *db.TableStats) {
	// Risk score formula:
	// - Lock type: 40 points (EXCLUSIVE=40, SHARE=20, NONE=0)
	// - Requires rewrite: 30 points
	// - Duration: 20 points (based on estimated time)
	// - Backward compatibility: 10 points (if not compatible)

	lockScore := 0
	switch op.LockType {
	case models.LockTypeExclusive:
		lockScore = 40
	case models.LockTypeShare:
		lockScore = 20
	case models.LockTypeNone:
		lockScore = 0
	default:
		lockScore = 10
	}

	rewriteScore := 0
	if op.RequiresRewrite {
		rewriteScore = 30
	}

	durationScore := 0
	estimatedMinutes := op.EstimatedTimeSeconds / 60.0
	durationScore = int(math.Min((estimatedMinutes/60.0)*20, 20))

	compatibilityScore := 0
	if !op.BackwardCompatible {
		compatibilityScore = 10
	}

	op.RiskScore = lockScore + rewriteScore + durationScore + compatibilityScore

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
		// Adding column with NOT NULL breaks compatibility
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
			op.Recommendations = append(op.Recommendations,
				"Consider using pt-online-schema-change for large tables to avoid locking")
		}
		sqlLower := strings.ToLower(op.SQL)
		if strings.Contains(sqlLower, "not null") && !strings.Contains(sqlLower, "default") {
			op.Recommendations = append(op.Recommendations,
				"Consider adding column as nullable first, then add NOT NULL constraint after backfilling")
		}

	case models.OperationTypeCreateIndex:
		if op.LockType == models.LockTypeExclusive {
			op.Recommendations = append(op.Recommendations,
				"Use ALGORITHM=INPLACE to avoid blocking writes during index creation")
		}
		if stats != nil && stats.TableSizeBytes > 1*1024*1024*1024 { // > 1 GB
			op.Recommendations = append(op.Recommendations,
				"For large tables, consider using pt-online-schema-change to build index online")
		}

	case models.OperationTypeAlterColumn:
		if op.RequiresRewrite {
			op.Recommendations = append(op.Recommendations,
				"Type changes require full table rewrite - consider creating new column, migrating data, then dropping old column")
			op.Recommendations = append(op.Recommendations,
				"Use pt-online-schema-change for large tables to avoid long-running locks")
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

	// MySQL-specific recommendations
	if op.RequiresRewrite && stats != nil && stats.TableSizeBytes > 5*1024*1024*1024 { // > 5 GB
		op.Recommendations = append(op.Recommendations,
			"For tables >5GB, strongly recommend pt-online-schema-change or gh-ost for zero-downtime migrations")
	}
}
