package concurrency

import (
	"github.com/iamsr/tapa/pkg/models"
)

// LockCalculator calculates lock impact for database operations
type LockCalculator struct {
	databaseType string
}

// NewLockCalculator creates a new lock calculator for the specified database type
func NewLockCalculator(databaseType string) *LockCalculator {
	return &LockCalculator{
		databaseType: databaseType,
	}
}

// CalculateImpact calculates the lock impact for an operation
func (lc *LockCalculator) CalculateImpact(op *models.Operation) *models.LockImpact {
	if op == nil {
		return nil
	}

	impact := &models.LockImpact{
		LockType:            op.LockType,
		EstimatedDurationMS: lc.EstimateLockDuration(op),
	}

	// Analyze lock behavior to determine what gets blocked
	lc.analyzeLockBehavior(impact, op)

	// Estimate how many queries will be blocked
	impact.EstimatedBlockedCount = lc.estimateBlockedQueries(op, impact)

	// Calculate human-readable wait time range
	impact.WaitTimeRange = lc.calculateWaitTimeRange(impact.EstimatedDurationMS)

	// Assess the risk of acquiring this lock
	impact.LockAcquisitionRisk = lc.assessLockAcquisitionRisk(op, impact)

	return impact
}

// EstimateLockDuration estimates the lock duration in milliseconds
func (lc *LockCalculator) EstimateLockDuration(op *models.Operation) int64 {
	// If we have an estimated time from the operation, use it
	if op.EstimatedTimeSeconds > 0 {
		return int64(op.EstimatedTimeSeconds * 1000)
	}

	// Otherwise use heuristics based on operation characteristics
	var baseMS int64

	// Start with row count based estimation
	switch {
	case op.RowCount > 10000000: // > 10M rows
		baseMS = 60000 // 60 seconds
	case op.RowCount > 1000000: // > 1M rows
		baseMS = 30000 // 30 seconds
	case op.RowCount > 100000: // > 100K rows
		baseMS = 10000 // 10 seconds
	case op.RowCount > 10000: // > 10K rows
		baseMS = 5000 // 5 seconds
	default:
		baseMS = 1000 // 1 second
	}

	// Adjust for operation type
	switch op.Type {
	case models.OperationTypeCreateIndex:
		// Index creation takes longer
		baseMS *= 3
	case models.OperationTypeAddColumn, models.OperationTypeAlterColumn:
		if op.RequiresRewrite {
			// Full table rewrite takes much longer
			baseMS *= 5
		}
	case models.OperationTypeDropColumn, models.OperationTypeDropIndex:
		// Drops are typically faster
		baseMS /= 2
	}

	// Minimum lock duration
	if baseMS < 100 {
		baseMS = 100
	}

	return baseMS
}

// analyzeLockBehavior sets blocking behavior based on lock type
func (lc *LockCalculator) analyzeLockBehavior(impact *models.LockImpact, op *models.Operation) {
	switch impact.LockType {
	case models.LockTypeAccessExclusive:
		// ACCESS EXCLUSIVE blocks everything
		impact.BlocksReads = true
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"SELECT", "INSERT", "UPDATE", "DELETE"}

	case models.LockTypeExclusive:
		// EXCLUSIVE blocks writes but not reads
		impact.BlocksReads = false
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"INSERT", "UPDATE", "DELETE"}

	case models.LockTypeShareUpdateExclusive:
		// SHARE UPDATE EXCLUSIVE - minimal blocking (used by CREATE INDEX CONCURRENTLY)
		impact.BlocksReads = false
		impact.BlocksWrites = false
		impact.BlockedQueryTypes = []string{} // Minimal impact

	case models.LockTypeShare:
		// SHARE allows reads, blocks some writes
		impact.BlocksReads = false
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"INSERT", "UPDATE", "DELETE", "ALTER TABLE"}

	case models.LockTypeRowExclusive:
		// ROW EXCLUSIVE - minimal impact, only blocks exclusive locks
		impact.BlocksReads = false
		impact.BlocksWrites = false
		impact.BlockedQueryTypes = []string{}

	default:
		// Conservative default: blocks writes
		impact.BlocksReads = false
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"INSERT", "UPDATE", "DELETE"}
	}
}

// estimateBlockedQueries estimates how many queries will be blocked
func (lc *LockCalculator) estimateBlockedQueries(op *models.Operation, impact *models.LockImpact) int {
	if len(impact.BlockedQueryTypes) == 0 {
		return 0
	}

	// Base estimate on duration and impact severity
	durationSeconds := float64(impact.EstimatedDurationMS) / 1000.0

	// Assume a baseline query rate (queries per second)
	// This is a rough heuristic - real implementation would use workload analysis
	var baseQPS float64
	if impact.BlocksReads {
		// If blocking reads, much higher query rate assumed
		baseQPS = 100.0
	} else if impact.BlocksWrites {
		// If only blocking writes, moderate query rate
		baseQPS = 20.0
	} else {
		// Minimal blocking
		baseQPS = 5.0
	}

	// Calculate estimated blocked queries
	estimatedBlocked := int(durationSeconds * baseQPS)

	return estimatedBlocked
}

// calculateWaitTimeRange converts duration to human-readable ranges
func (lc *LockCalculator) calculateWaitTimeRange(durationMS int64) string {
	seconds := durationMS / 1000

	switch {
	case seconds < 1:
		return "< 1 second"
	case seconds < 2:
		return "1-2 seconds"
	case seconds < 5:
		return "2-5 seconds"
	case seconds < 10:
		return "5-10 seconds"
	case seconds < 30:
		return "10-30 seconds"
	case seconds < 60:
		return "30-60 seconds"
	case seconds < 300: // 5 minutes
		return "1-5 minutes"
	case seconds < 600: // 10 minutes
		return "5-10 minutes"
	default:
		return "> 10 minutes"
	}
}

// assessLockAcquisitionRisk assesses the risk of acquiring the lock
func (lc *LockCalculator) assessLockAcquisitionRisk(op *models.Operation, impact *models.LockImpact) string {
	// Start with base risk from lock type
	var riskScore int

	switch impact.LockType {
	case models.LockTypeAccessExclusive:
		riskScore = 80 // Very high base risk
	case models.LockTypeExclusive:
		riskScore = 60 // High base risk
	case models.LockTypeShare:
		riskScore = 40 // Medium base risk
	case models.LockTypeShareUpdateExclusive:
		riskScore = 20 // Low base risk
	case models.LockTypeRowExclusive:
		riskScore = 10 // Very low base risk
	default:
		riskScore = 50 // Medium default
	}

	// Adjust for duration
	durationSeconds := impact.EstimatedDurationMS / 1000
	if durationSeconds > 60 {
		riskScore += 20 // Long duration increases risk
	} else if durationSeconds > 30 {
		riskScore += 10
	}

	// Adjust for table size
	if op.RowCount > 1000000 {
		riskScore += 10 // Large tables increase risk
	}

	// Cap at 100
	if riskScore > 100 {
		riskScore = 100
	}

	// Convert score to category
	switch {
	case riskScore >= 80:
		return "critical"
	case riskScore >= 60:
		return "high"
	case riskScore >= 40:
		return "medium"
	default:
		return "low"
	}
}
