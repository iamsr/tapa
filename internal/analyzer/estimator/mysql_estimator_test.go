package estimator

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

func TestMySQLEstimator_EstimateTime_WithRewrite(t *testing.T) {
	estimator := newMySQLTimeEstimator(nil, 100, 2.0)

	op := &models.Operation{
		Type:            models.OperationTypeAddColumn,
		TableName:       "users",
		RequiresRewrite: true,
	}

	// Mock table stats
	stats := &db.TableStats{
		TableName:      "users",
		RowCount:       1000000,
		TableSizeBytes: 500 * 1024 * 1024, // 500 MB
		Indexes: []db.IndexInfo{
			{IndexName: "idx_email", Columns: []string{"email"}},
			{IndexName: "idx_status", Columns: []string{"status"}},
		},
	}

	// Test with stats
	breakdown, err := estimator.estimateTimeWithStats(context.Background(), op, stats)
	if err != nil {
		t.Fatalf("estimateTimeWithStats failed: %v", err)
	}

	if breakdown.TotalSeconds <= 0 {
		t.Error("Expected positive time estimate for table rewrite")
	}

	if breakdown.TableRewriteSeconds <= 0 {
		t.Error("Expected positive table rewrite time")
	}

	if breakdown.IndexBuildSeconds <= 0 {
		t.Error("Expected positive index build time")
	}
}

func TestMySQLEstimator_EstimateTime_CreateIndex(t *testing.T) {
	estimator := newMySQLTimeEstimator(nil, 100, 2.0)

	op := &models.Operation{
		Type:            models.OperationTypeCreateIndex,
		TableName:       "users",
		RequiresRewrite: false,
	}

	stats := &db.TableStats{
		TableName:      "users",
		RowCount:       1000000,
		TableSizeBytes: 500 * 1024 * 1024, // 500 MB
	}

	breakdown, err := estimator.estimateTimeWithStats(context.Background(), op, stats)
	if err != nil {
		t.Fatalf("estimateTimeWithStats failed: %v", err)
	}

	if breakdown.TotalSeconds <= 0 {
		t.Error("Expected positive time estimate for CREATE INDEX")
	}

	if breakdown.IndexBuildSeconds <= 0 {
		t.Error("Expected positive index build time")
	}

	if breakdown.TableRewriteSeconds > 0 {
		t.Error("Expected no table rewrite for CREATE INDEX")
	}
}

func TestMySQLEstimator_EstimateTime_NoIntrospector(t *testing.T) {
	estimator := newMySQLTimeEstimator(nil, 100, 2.0)

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
	}

	breakdown, err := estimator.EstimateTime(context.Background(), op)
	if err != nil {
		t.Fatalf("EstimateTime failed: %v", err)
	}

	if breakdown.TotalSeconds <= 0 {
		t.Error("Expected positive time estimate even without introspector")
	}

	if breakdown.MetadataUpdateSeconds <= 0 {
		t.Error("Expected metadata update time")
	}
}

func TestMySQLEstimator_EstimateTime_NoTableName(t *testing.T) {
	estimator := newMySQLTimeEstimator(nil, 100, 2.0)

	op := &models.Operation{
		Type: models.OperationTypeAlterTable,
	}

	breakdown, err := estimator.EstimateTime(context.Background(), op)
	if err != nil {
		t.Fatalf("EstimateTime failed: %v", err)
	}

	if breakdown.TotalSeconds <= 0 {
		t.Error("Expected positive time estimate")
	}

	if breakdown.MetadataUpdateSeconds <= 0 {
		t.Error("Expected metadata update time")
	}
}
