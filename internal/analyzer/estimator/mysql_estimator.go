package estimator

import (
	"context"

	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

// mysqlTimeEstimator implements TimeEstimator for MySQL
type mysqlTimeEstimator struct {
	introspector   db.Introspector
	throughputMBps int
	rewriteFactor  float64
}

// newMySQLTimeEstimator creates a MySQL time estimator
func newMySQLTimeEstimator(introspector db.Introspector, throughputMBps int, rewriteFactor float64) *mysqlTimeEstimator {
	// Validate and set defaults for invalid inputs
	if throughputMBps <= 0 {
		throughputMBps = 100 // reasonable default: 100 MB/s
	}
	if rewriteFactor <= 0 {
		rewriteFactor = 2.0 // reasonable default
	}

	return &mysqlTimeEstimator{
		introspector:   introspector,
		throughputMBps: throughputMBps,
		rewriteFactor:  rewriteFactor,
	}
}

// EstimateTime calculates time breakdown
func (e *mysqlTimeEstimator) EstimateTime(ctx context.Context, op *models.Operation) (*models.TimeBreakdown, error) {
	breakdown := &models.TimeBreakdown{}

	// If no table name or introspector, return minimal estimate
	if op.TableName == "" || e.introspector == nil {
		breakdown.MetadataUpdateSeconds = 0.1
		breakdown.CalculateTotal()
		return breakdown, nil
	}

	// Get table stats
	stats, err := e.introspector.GetTableStats(ctx, op.TableName)
	if err != nil {
		// If can't get stats, use conservative estimate
		breakdown.MetadataUpdateSeconds = 0.1
		breakdown.CalculateTotal()
		return breakdown, nil
	}

	return e.estimateTimeWithStats(ctx, op, stats)
}

// estimateTimeWithStats calculates time with provided stats
func (e *mysqlTimeEstimator) estimateTimeWithStats(ctx context.Context, op *models.Operation, stats *db.TableStats) (*models.TimeBreakdown, error) {
	breakdown := &models.TimeBreakdown{}

	if op.RequiresRewrite {
		// Table copy time (MySQL copies entire table for ALTER TABLE operations)
		breakdown.TableRewriteSeconds = e.calculateRewriteTime(stats)

		// Index rebuild time (MySQL rebuilds all indexes after table copy)
		breakdown.IndexBuildSeconds = e.calculateIndexRebuildTime(stats)
	} else if op.Type == models.OperationTypeCreateIndex {
		// Index creation time (building index on existing table)
		breakdown.IndexBuildSeconds = e.calculateIndexCreateTime(stats)
	}

	// Metadata updates are fast
	breakdown.MetadataUpdateSeconds = 0.1

	breakdown.CalculateTotal()
	return breakdown, nil
}

// calculateRewriteTime estimates table copy duration
func (e *mysqlTimeEstimator) calculateRewriteTime(stats *db.TableStats) float64 {
	tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
	baseTime := tableSizeMB / float64(e.throughputMBps)
	return baseTime * e.rewriteFactor
}

// calculateIndexRebuildTime estimates time to rebuild all indexes after table copy
func (e *mysqlTimeEstimator) calculateIndexRebuildTime(stats *db.TableStats) float64 {
	indexCount := len(stats.Indexes)
	if indexCount == 0 {
		return 0
	}

	tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
	baseTimePerIndex := tableSizeMB / float64(e.throughputMBps)

	// Indexes are typically smaller and faster to build
	return baseTimePerIndex * float64(indexCount) * 0.5
}

// calculateIndexCreateTime estimates time to create a new index
func (e *mysqlTimeEstimator) calculateIndexCreateTime(stats *db.TableStats) float64 {
	tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
	baseTime := tableSizeMB / float64(e.throughputMBps)

	// Creating an index requires reading the table and building the index
	// More expensive than rebuilding during a table copy
	return baseTime * 1.5
}
