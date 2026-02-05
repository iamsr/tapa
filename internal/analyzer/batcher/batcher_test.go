package batcher

import (
	"testing"

	"github.com/yourusername/dma/pkg/models"
)

// TestGetMigrationBatcher tests the factory pattern
func TestGetMigrationBatcher(t *testing.T) {
	tests := []struct {
		name        string
		dbType      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "postgresql returns batcher",
			dbType:      "postgresql",
			expectError: false,
		},
		{
			name:        "mysql not implemented",
			dbType:      "mysql",
			expectError: true,
			errorMsg:    "MySQL batcher not yet implemented",
		},
		{
			name:        "unsupported database type",
			dbType:      "oracle",
			expectError: true,
			errorMsg:    "unsupported database type: oracle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batcher, err := GetMigrationBatcher(tt.dbType)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
				if batcher != nil {
					t.Errorf("expected nil batcher on error, got %T", batcher)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if batcher == nil {
					t.Errorf("expected batcher, got nil")
				}
			}
		})
	}
}

// TestPostgresBatcher_GenerateBatches_EmptyOperations tests edge case
func TestPostgresBatcher_GenerateBatches_EmptyOperations(t *testing.T) {
	batcher, err := GetMigrationBatcher("postgresql")
	if err != nil {
		t.Fatalf("failed to create batcher: %v", err)
	}

	ops := []*models.Operation{}
	strategy, err := batcher.GenerateBatches(ops)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if strategy == nil {
		t.Fatal("expected strategy, got nil")
	}
	if strategy.TotalBatches != 0 {
		t.Errorf("expected 0 batches, got %d", strategy.TotalBatches)
	}
	if len(strategy.Batches) != 0 {
		t.Errorf("expected empty batches, got %d", len(strategy.Batches))
	}
}

// TestPostgresBatcher_GenerateBatches_LowRiskOnly tests low-risk operations
func TestPostgresBatcher_GenerateBatches_LowRiskOnly(t *testing.T) {
	batcher, err := GetMigrationBatcher("postgresql")
	if err != nil {
		t.Fatalf("failed to create batcher: %v", err)
	}

	ops := []*models.Operation{
		{SQL: "CREATE INDEX idx1 ON users(email);", RiskScore: 10, EstimatedTimeSeconds: 5.0},
		{SQL: "CREATE INDEX idx2 ON posts(created_at);", RiskScore: 15, EstimatedTimeSeconds: 3.0},
		{SQL: "CREATE TABLE logs(...);", RiskScore: 5, EstimatedTimeSeconds: 1.0},
	}

	strategy, err := batcher.GenerateBatches(ops)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 batch with all operations
	if strategy.TotalBatches != 1 {
		t.Errorf("expected 1 batch, got %d", strategy.TotalBatches)
	}

	batch := strategy.Batches[0]
	if batch.BatchNumber != 1 {
		t.Errorf("expected batch number 1, got %d", batch.BatchNumber)
	}
	if len(batch.Operations) != 3 {
		t.Errorf("expected 3 operations, got %d", len(batch.Operations))
	}
	if !batch.CanRunInParallel {
		t.Errorf("expected CanRunInParallel=true for low-risk batch")
	}
	if len(batch.Prerequisites) != 0 {
		t.Errorf("expected no prerequisites, got %v", batch.Prerequisites)
	}
	if batch.Rationale != "Low-risk operations can be deployed immediately" {
		t.Errorf("wrong rationale: %s", batch.Rationale)
	}
	// Check that metrics were calculated
	if batch.MaxRiskScore != 15 {
		t.Errorf("expected max risk 15, got %d", batch.MaxRiskScore)
	}
	if batch.TotalTimeSeconds != 9.0 {
		t.Errorf("expected total time 9.0s, got %.1f", batch.TotalTimeSeconds)
	}
}

// TestPostgresBatcher_GenerateBatches_MediumRiskOnly tests medium-risk operations
func TestPostgresBatcher_GenerateBatches_MediumRiskOnly(t *testing.T) {
	batcher, err := GetMigrationBatcher("postgresql")
	if err != nil {
		t.Fatalf("failed to create batcher: %v", err)
	}

	ops := []*models.Operation{
		{SQL: "ALTER TABLE users ADD COLUMN age INT;", RiskScore: 30, EstimatedTimeSeconds: 10.0},
		{SQL: "ALTER TABLE posts ADD COLUMN views INT;", RiskScore: 35, EstimatedTimeSeconds: 8.0},
	}

	strategy, err := batcher.GenerateBatches(ops)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 batch with all medium-risk operations
	if strategy.TotalBatches != 1 {
		t.Errorf("expected 1 batch, got %d", strategy.TotalBatches)
	}

	batch := strategy.Batches[0]
	if batch.BatchNumber != 1 {
		t.Errorf("expected batch number 1, got %d", batch.BatchNumber)
	}
	if batch.CanRunInParallel {
		t.Errorf("expected CanRunInParallel=false for medium-risk batch")
	}
	if batch.Rationale != "Medium-risk operations should be deployed during low-traffic periods" {
		t.Errorf("wrong rationale: %s", batch.Rationale)
	}
}

// TestPostgresBatcher_GenerateBatches_HighRiskIsolation tests high-risk operations are isolated
func TestPostgresBatcher_GenerateBatches_HighRiskIsolation(t *testing.T) {
	batcher, err := GetMigrationBatcher("postgresql")
	if err != nil {
		t.Fatalf("failed to create batcher: %v", err)
	}

	ops := []*models.Operation{
		{SQL: "ALTER TABLE users DROP COLUMN legacy_field;", RiskScore: 60, EstimatedTimeSeconds: 120.0},
		{SQL: "DROP TABLE old_logs;", RiskScore: 80, EstimatedTimeSeconds: 30.0},
	}

	strategy, err := batcher.GenerateBatches(ops)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 batches, one per high-risk operation
	if strategy.TotalBatches != 2 {
		t.Errorf("expected 2 batches, got %d", strategy.TotalBatches)
	}

	// First high-risk operation
	batch1 := strategy.Batches[0]
	if len(batch1.Operations) != 1 {
		t.Errorf("expected 1 operation in batch 1, got %d", len(batch1.Operations))
	}
	if batch1.CanRunInParallel {
		t.Errorf("expected CanRunInParallel=false for high-risk batch")
	}
	if batch1.Rationale != "High-risk operation requires maintenance window" {
		t.Errorf("wrong rationale for batch 1: %s", batch1.Rationale)
	}

	// Second high-risk operation
	batch2 := strategy.Batches[1]
	if len(batch2.Operations) != 1 {
		t.Errorf("expected 1 operation in batch 2, got %d", len(batch2.Operations))
	}
	if batch2.Rationale != "High-risk operation requires maintenance window" {
		t.Errorf("wrong rationale for batch 2: %s", batch2.Rationale)
	}
	// Second batch should depend on first
	if len(batch2.Prerequisites) != 1 || batch2.Prerequisites[0] != 1 {
		t.Errorf("expected prerequisites [1], got %v", batch2.Prerequisites)
	}
}

// TestPostgresBatcher_GenerateBatches_MixedRisk tests mixed risk levels
func TestPostgresBatcher_GenerateBatches_MixedRisk(t *testing.T) {
	batcher, err := GetMigrationBatcher("postgresql")
	if err != nil {
		t.Fatalf("failed to create batcher: %v", err)
	}

	ops := []*models.Operation{
		{SQL: "CREATE INDEX idx1 ON users(email);", RiskScore: 10, EstimatedTimeSeconds: 5.0},
		{SQL: "CREATE INDEX idx2 ON posts(created_at);", RiskScore: 15, EstimatedTimeSeconds: 3.0},
		{SQL: "ALTER TABLE users ADD COLUMN age INT;", RiskScore: 30, EstimatedTimeSeconds: 10.0},
		{SQL: "ALTER TABLE posts ADD COLUMN views INT;", RiskScore: 35, EstimatedTimeSeconds: 8.0},
		{SQL: "ALTER TABLE users DROP COLUMN legacy_field;", RiskScore: 60, EstimatedTimeSeconds: 120.0},
		{SQL: "DROP TABLE old_logs;", RiskScore: 80, EstimatedTimeSeconds: 30.0},
	}

	strategy, err := batcher.GenerateBatches(ops)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 batches: low, medium, high1, high2
	if strategy.TotalBatches != 4 {
		t.Errorf("expected 4 batches, got %d", strategy.TotalBatches)
	}

	// Batch 1: Low-risk operations (2 ops)
	batch1 := strategy.Batches[0]
	if batch1.BatchNumber != 1 {
		t.Errorf("expected batch number 1, got %d", batch1.BatchNumber)
	}
	if len(batch1.Operations) != 2 {
		t.Errorf("expected 2 operations in batch 1, got %d", len(batch1.Operations))
	}
	if !batch1.CanRunInParallel {
		t.Errorf("expected CanRunInParallel=true for batch 1")
	}
	if len(batch1.Prerequisites) != 0 {
		t.Errorf("expected no prerequisites for batch 1, got %v", batch1.Prerequisites)
	}

	// Batch 2: Medium-risk operations (2 ops)
	batch2 := strategy.Batches[1]
	if batch2.BatchNumber != 2 {
		t.Errorf("expected batch number 2, got %d", batch2.BatchNumber)
	}
	if len(batch2.Operations) != 2 {
		t.Errorf("expected 2 operations in batch 2, got %d", len(batch2.Operations))
	}
	if batch2.CanRunInParallel {
		t.Errorf("expected CanRunInParallel=false for batch 2")
	}
	if len(batch2.Prerequisites) != 1 || batch2.Prerequisites[0] != 1 {
		t.Errorf("expected prerequisites [1] for batch 2, got %v", batch2.Prerequisites)
	}

	// Batch 3: First high-risk operation (1 op)
	batch3 := strategy.Batches[2]
	if batch3.BatchNumber != 3 {
		t.Errorf("expected batch number 3, got %d", batch3.BatchNumber)
	}
	if len(batch3.Operations) != 1 {
		t.Errorf("expected 1 operation in batch 3, got %d", len(batch3.Operations))
	}
	if batch3.CanRunInParallel {
		t.Errorf("expected CanRunInParallel=false for batch 3")
	}
	if len(batch3.Prerequisites) != 2 || batch3.Prerequisites[0] != 1 || batch3.Prerequisites[1] != 2 {
		t.Errorf("expected prerequisites [1, 2] for batch 3, got %v", batch3.Prerequisites)
	}

	// Batch 4: Second high-risk operation (1 op)
	batch4 := strategy.Batches[3]
	if batch4.BatchNumber != 4 {
		t.Errorf("expected batch number 4, got %d", batch4.BatchNumber)
	}
	if len(batch4.Operations) != 1 {
		t.Errorf("expected 1 operation in batch 4, got %d", len(batch4.Operations))
	}
	if len(batch4.Prerequisites) != 3 || batch4.Prerequisites[0] != 1 || batch4.Prerequisites[1] != 2 || batch4.Prerequisites[2] != 3 {
		t.Errorf("expected prerequisites [1, 2, 3] for batch 4, got %v", batch4.Prerequisites)
	}
}

// TestPostgresBatcher_GenerateBatches_Recommendations tests recommendations are generated
func TestPostgresBatcher_GenerateBatches_Recommendations(t *testing.T) {
	batcher, err := GetMigrationBatcher("postgresql")
	if err != nil {
		t.Fatalf("failed to create batcher: %v", err)
	}

	// Multiple high-risk operations
	ops := []*models.Operation{
		{SQL: "DROP TABLE old_logs;", RiskScore: 80, EstimatedTimeSeconds: 30.0},
		{SQL: "ALTER TABLE users DROP COLUMN legacy;", RiskScore: 70, EstimatedTimeSeconds: 120.0},
		{SQL: "DROP INDEX idx_old;", RiskScore: 55, EstimatedTimeSeconds: 10.0},
	}

	strategy, err := batcher.GenerateBatches(ops)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have recommendations
	if len(strategy.Recommendations) == 0 {
		t.Fatal("expected recommendations, got none")
	}

	// Should mention batch count
	foundBatchCount := false
	foundBatch1Warning := false
	foundBatch2Warning := false
	foundBatch3Warning := false
	for _, rec := range strategy.Recommendations {
		if rec == "💡 Split into 3 batches for safer deployment" {
			foundBatchCount = true
		}
		if rec == "⚠️  Batch 1 (CRITICAL): Deploy during maintenance window" {
			foundBatch1Warning = true
		}
		if rec == "⚠️  Batch 2 (HIGH): Deploy during maintenance window" {
			foundBatch2Warning = true
		}
		if rec == "⚠️  Batch 3 (HIGH): Deploy during maintenance window" {
			foundBatch3Warning = true
		}
	}

	if !foundBatchCount {
		t.Errorf("expected batch count recommendation, got: %v", strategy.Recommendations)
	}
	if !foundBatch1Warning {
		t.Errorf("expected batch 1 maintenance window warning, got: %v", strategy.Recommendations)
	}
	if !foundBatch2Warning {
		t.Errorf("expected batch 2 maintenance window warning, got: %v", strategy.Recommendations)
	}
	if !foundBatch3Warning {
		t.Errorf("expected batch 3 maintenance window warning, got: %v", strategy.Recommendations)
	}
}
