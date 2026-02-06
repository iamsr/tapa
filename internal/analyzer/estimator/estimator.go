package estimator

import (
	"context"
	"fmt"

	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

// TimeEstimator calculates detailed time estimates for operations
type TimeEstimator interface {
	// EstimateTime calculates time breakdown for an operation
	EstimateTime(ctx context.Context, op *models.Operation) (*models.TimeBreakdown, error)
}

// GetTimeEstimator returns appropriate estimator for database type
func GetTimeEstimator(dbType string, introspector db.Introspector, throughputMBps int, rewriteFactor float64) (TimeEstimator, error) {
	switch dbType {
	case "postgresql":
		return newPostgresTimeEstimator(introspector, throughputMBps, rewriteFactor), nil
	case "mysql":
		return newMySQLTimeEstimator(introspector, throughputMBps, rewriteFactor), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// postgresTimeEstimator implements TimeEstimator for PostgreSQL
type postgresTimeEstimator struct {
	introspector   db.Introspector
	throughputMBps int
	rewriteFactor  float64
}

func newPostgresTimeEstimator(introspector db.Introspector, throughputMBps int, rewriteFactor float64) *postgresTimeEstimator {
	// Validate and set defaults for invalid inputs
	if throughputMBps <= 0 {
		throughputMBps = 100 // reasonable default: 100 MB/s
	}
	if rewriteFactor <= 0 {
		rewriteFactor = 2.0 // reasonable default
	}

	return &postgresTimeEstimator{
		introspector:   introspector,
		throughputMBps: throughputMBps,
		rewriteFactor:  rewriteFactor,
	}
}

// EstimateTime calculates time breakdown
func (e *postgresTimeEstimator) EstimateTime(ctx context.Context, op *models.Operation) (*models.TimeBreakdown, error) {
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

	// Calculate components based on operation type
	if op.RequiresRewrite {
		breakdown.TableRewriteSeconds = e.calculateRewriteTime(stats)
		breakdown.IndexBuildSeconds = e.calculateIndexRebuildTime(stats)
	}

	// Metadata updates are fast
	breakdown.MetadataUpdateSeconds = 0.1

	breakdown.CalculateTotal()
	return breakdown, nil
}

func (e *postgresTimeEstimator) calculateRewriteTime(stats *db.TableStats) float64 {
	tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
	baseTime := tableSizeMB / float64(e.throughputMBps)
	return baseTime * e.rewriteFactor
}

func (e *postgresTimeEstimator) calculateIndexRebuildTime(stats *db.TableStats) float64 {
	// Simple estimate: rebuilding all indexes takes about same time as table rewrite
	// More sophisticated: account for index type, column count, etc.
	indexCount := len(stats.Indexes)
	if indexCount == 0 {
		return 0
	}

	tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
	baseTimePerIndex := tableSizeMB / float64(e.throughputMBps)

	// Indexes are typically smaller and faster to build
	return baseTimePerIndex * float64(indexCount) * 0.5
}
