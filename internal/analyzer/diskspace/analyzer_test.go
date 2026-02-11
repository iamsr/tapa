package diskspace_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/diskspace"
	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAnalyzer_AnalyzeDiskSpace_TableRewrite(t *testing.T) {
	analyzer := diskspace.NewAnalyzer("postgresql", 200)

	op := &models.Operation{
		Type:            models.OperationTypeAlterColumn,
		TableName:       "users",
		RequiresRewrite: true,
	}

	stats := &db.TableStats{
		TableSizeBytes: 80 * GB,
		IndexSizeBytes: 15 * GB,
		RowCount:       10000000,
	}

	err := analyzer.AnalyzeDiskSpace(context.Background(), op, stats)
	if err != nil {
		t.Fatalf("AnalyzeDiskSpace failed: %v", err)
	}

	if op.DiskSpaceAnalysis == nil {
		t.Fatal("DiskSpaceAnalysis is nil")
	}

	// Should require approximately 2x table size for rewrite
	expectedPeak := int64(float64(stats.TableSizeBytes) * 2.0)
	tolerance := int64(float64(expectedPeak) * 0.1) // 10% tolerance

	if op.DiskSpaceAnalysis.MigrationRequirements.PeakDiskUsageBytes < expectedPeak-tolerance ||
		op.DiskSpaceAnalysis.MigrationRequirements.PeakDiskUsageBytes > expectedPeak+tolerance {
		t.Errorf("Peak disk usage = %d, want ~%d",
			op.DiskSpaceAnalysis.MigrationRequirements.PeakDiskUsageBytes,
			expectedPeak)
	}
}

func TestAnalyzer_AnalyzeDiskSpace_CreateIndex(t *testing.T) {
	analyzer := diskspace.NewAnalyzer("postgresql", 200)

	op := &models.Operation{
		Type:      models.OperationTypeCreateIndex,
		TableName: "users",
		IndexName: "idx_users_email",
	}

	stats := &db.TableStats{
		TableSizeBytes: 50 * GB,
		IndexSizeBytes: 10 * GB,
	}

	err := analyzer.AnalyzeDiskSpace(context.Background(), op, stats)
	if err != nil {
		t.Fatalf("AnalyzeDiskSpace failed: %v", err)
	}

	// Index creation should estimate new index size
	if op.DiskSpaceAnalysis.MigrationRequirements.NewIndexBytes == 0 {
		t.Error("NewIndexBytes should be > 0 for CREATE INDEX")
	}
}

const GB = 1024 * 1024 * 1024
