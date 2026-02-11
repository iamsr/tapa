package datamigration_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/datamigration"
	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAnalyzer_DetectDataMigration_Update(t *testing.T) {
	analyzer := datamigration.NewAnalyzer("postgresql")

	op := &models.Operation{
		Type:      models.OperationTypeAlterTable,
		TableName: "users",
		SQL:       "UPDATE users SET full_name = first_name || ' ' || last_name WHERE full_name IS NULL",
	}

	stats := &db.TableStats{
		RowCount: 5000000,
	}

	err := analyzer.DetectDataMigration(context.Background(), op, stats)
	if err != nil {
		t.Fatalf("DetectDataMigration failed: %v", err)
	}

	if op.DataMigrationAnalysis == nil {
		t.Fatal("DataMigrationAnalysis is nil")
	}

	if !op.DataMigrationAnalysis.HasDataMigration {
		t.Error("Should detect data migration")
	}

	if op.DataMigrationAnalysis.OperationType != "UPDATE" {
		t.Errorf("OperationType = %v, want UPDATE", op.DataMigrationAnalysis.OperationType)
	}

	if op.DataMigrationAnalysis.Complexity != models.DataMigrationSimple {
		t.Errorf("Complexity = %v, want SIMPLE_COMPUTATION", op.DataMigrationAnalysis.Complexity)
	}
}

func TestAnalyzer_DetectDataMigration_NoDataMigration(t *testing.T) {
	analyzer := datamigration.NewAnalyzer("postgresql")

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
	}

	stats := &db.TableStats{
		RowCount: 5000000,
	}

	err := analyzer.DetectDataMigration(context.Background(), op, stats)
	if err != nil {
		t.Fatalf("DetectDataMigration failed: %v", err)
	}

	// Should be nil or HasDataMigration = false
	if op.DataMigrationAnalysis != nil && op.DataMigrationAnalysis.HasDataMigration {
		t.Error("Should not detect data migration for simple ADD COLUMN")
	}
}

func TestAnalyzer_DetectDataMigration_BatchingRecommendation(t *testing.T) {
	analyzer := datamigration.NewAnalyzer("postgresql")

	op := &models.Operation{
		Type:      models.OperationTypeAlterTable,
		TableName: "users",
		SQL:       "UPDATE users SET status = 'active' WHERE status IS NULL",
	}

	stats := &db.TableStats{
		RowCount: 10000000, // 10M rows
	}

	err := analyzer.DetectDataMigration(context.Background(), op, stats)
	if err != nil {
		t.Fatalf("DetectDataMigration failed: %v", err)
	}

	if op.DataMigrationAnalysis.BatchingRecommendation == nil {
		t.Fatal("BatchingRecommendation is nil")
	}

	if !op.DataMigrationAnalysis.BatchingRecommendation.ShouldBatch {
		t.Error("Should recommend batching for 10M rows")
	}

	if op.DataMigrationAnalysis.BatchingRecommendation.RecommendedBatchSize == 0 {
		t.Error("RecommendedBatchSize should be > 0")
	}
}
