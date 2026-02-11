package diskspace

import (
	"context"

	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

// Analyzer analyzes disk space requirements for PostgreSQL operations
type Analyzer struct {
	databaseType       string
	diskThroughputMBps int
	safetyBufferPct    float64
}

// NewAnalyzer creates a new disk space analyzer
func NewAnalyzer(databaseType string, diskThroughputMBps int) *Analyzer {
	return &Analyzer{
		databaseType:       databaseType,
		diskThroughputMBps: diskThroughputMBps,
		safetyBufferPct:    0.20, // 20% safety buffer
	}
}

// AnalyzeDiskSpace analyzes disk space requirements for an operation
func (a *Analyzer) AnalyzeDiskSpace(ctx context.Context, op *models.Operation, stats *db.TableStats) error {
	if stats == nil {
		return nil
	}

	// Create analysis structure
	analysis := &models.DiskSpaceAnalysis{}

	// Populate current state
	analysis.CurrentState = models.DiskSpaceState{
		TableSizeBytes: stats.TableSizeBytes,
		IndexSizeBytes: stats.IndexSizeBytes,
		TotalSizeBytes: stats.TableSizeBytes + stats.IndexSizeBytes,
	}

	// Calculate requirements based on operation type
	switch op.Type {
	case models.OperationTypeAlterColumn, models.OperationTypeAlterTable:
		if op.RequiresRewrite {
			analysis.MigrationRequirements = a.calculateRewriteRequirements(stats)
		} else {
			analysis.MigrationRequirements = a.calculateMetadataOnlyRequirements()
		}
	case models.OperationTypeCreateIndex:
		analysis.MigrationRequirements = a.calculateIndexCreationRequirements(stats)
	case models.OperationTypeAddColumn:
		if op.RequiresRewrite {
			analysis.MigrationRequirements = a.calculateRewriteRequirements(stats)
		} else {
			analysis.MigrationRequirements = a.calculateMetadataOnlyRequirements()
		}
	case models.OperationTypeDropColumn:
		// DROP COLUMN doesn't free space immediately in PostgreSQL
		analysis.MigrationRequirements = a.calculateMetadataOnlyRequirements()
	case models.OperationTypeDropIndex, models.OperationTypeDropTable:
		analysis.MigrationRequirements = a.calculateDropRequirements()
	default:
		analysis.MigrationRequirements = a.calculateMetadataOnlyRequirements()
	}

	// Calculate final state
	analysis.FinalState = a.calculateFinalState(analysis.CurrentState, analysis.MigrationRequirements, op)

	// Set system check (conservative - assumes 2x available space requirement)
	analysis.SystemCheck = models.SystemDiskCheck{
		AvailableBytes:     analysis.MigrationRequirements.PeakDiskUsageBytes * 2,
		RequiredBytes:      analysis.MigrationRequirements.PeakDiskUsageBytes,
		HasSufficientSpace: true, // Conservative default
		WarningThreshold:   0.8,
	}

	op.DiskSpaceAnalysis = analysis
	return nil
}

// calculateRewriteRequirements calculates space for table rewrites
func (a *Analyzer) calculateRewriteRequirements(stats *db.TableStats) models.MigrationDiskRequirements {
	// PostgreSQL creates a new table during rewrite (MVCC)
	// Peak usage: original table + temporary table copy (2x table size)
	temporaryTableBytes := stats.TableSizeBytes
	peakUsage := stats.TableSizeBytes + temporaryTableBytes

	// Calculate safety buffer (included in peak calculation)
	safetyBuffer := int64(float64(stats.TableSizeBytes+temporaryTableBytes) * a.safetyBufferPct)

	return models.MigrationDiskRequirements{
		RequiresRewrite:     true,
		TemporaryTableBytes: temporaryTableBytes,
		PeakDiskUsageBytes:  peakUsage,
		SafetyBufferBytes:   safetyBuffer,
	}
}

// calculateIndexCreationRequirements calculates space for index creation
func (a *Analyzer) calculateIndexCreationRequirements(stats *db.TableStats) models.MigrationDiskRequirements {
	// Estimate new index size as ~15% of table size
	newIndexBytes := int64(float64(stats.TableSizeBytes) * 0.15)

	// Peak usage: current table + indexes + new index
	peakUsage := stats.TableSizeBytes + stats.IndexSizeBytes + newIndexBytes

	// Calculate safety buffer (included in peak calculation)
	safetyBuffer := int64(float64(peakUsage) * a.safetyBufferPct)

	return models.MigrationDiskRequirements{
		RequiresRewrite:    false,
		NewIndexBytes:      newIndexBytes,
		PeakDiskUsageBytes: peakUsage,
		SafetyBufferBytes:  safetyBuffer,
	}
}

// calculateMetadataOnlyRequirements calculates space for metadata-only operations
func (a *Analyzer) calculateMetadataOnlyRequirements() models.MigrationDiskRequirements {
	// Metadata-only changes require minimal space (catalog updates, WAL)
	minimalSpace := int64(1024 * 1024) // 1MB

	return models.MigrationDiskRequirements{
		RequiresRewrite:    false,
		PeakDiskUsageBytes: minimalSpace,
		SafetyBufferBytes:  0,
	}
}

// calculateDropRequirements calculates space for DROP operations
func (a *Analyzer) calculateDropRequirements() models.MigrationDiskRequirements {
	// DROP operations don't require additional space
	return models.MigrationDiskRequirements{
		RequiresRewrite:    false,
		PeakDiskUsageBytes: 0,
		SafetyBufferBytes:  0,
	}
}

// calculateFinalState calculates disk usage after migration
func (a *Analyzer) calculateFinalState(current models.DiskSpaceState, requirements models.MigrationDiskRequirements, op *models.Operation) models.DiskSpaceState {
	final := current

	switch op.Type {
	case models.OperationTypeCreateIndex:
		final.IndexSizeBytes += requirements.NewIndexBytes
		final.TotalSizeBytes = final.TableSizeBytes + final.IndexSizeBytes
	case models.OperationTypeDropIndex:
		// Approximate - we don't know the exact index size being dropped
		final.IndexSizeBytes = max(0, final.IndexSizeBytes-int64(float64(final.IndexSizeBytes)*0.2))
		final.TotalSizeBytes = final.TableSizeBytes + final.IndexSizeBytes
	case models.OperationTypeDropTable:
		final.TableSizeBytes = 0
		final.IndexSizeBytes = 0
		final.TotalSizeBytes = 0
	default:
		// For most operations, final state is same as current
		final.TotalSizeBytes = final.TableSizeBytes + final.IndexSizeBytes
	}

	return final
}

// max returns the maximum of two int64 values
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
