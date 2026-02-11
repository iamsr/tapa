package dryrun_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/dryrun"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAnalyzer_AnalyzeOperation(t *testing.T) {
	analyzer := dryrun.NewAnalyzer("postgresql", nil)

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
	}

	ctx := context.Background()
	err := analyzer.AnalyzeOperation(ctx, op)
	if err != nil {
		t.Fatalf("AnalyzeOperation failed: %v", err)
	}

	if op.DryRunResult == nil {
		t.Fatal("DryRunResult should be populated")
	}

	// In mock mode without DB, should return skipped or success
	if op.DryRunResult.Status != models.DryRunStatusSuccess && op.DryRunResult.Status != models.DryRunStatusSkipped {
		t.Errorf("Expected success or skipped, got %s", op.DryRunResult.Status)
	}
}

func TestAnalyzer_AnalyzeMigration(t *testing.T) {
	analyzer := dryrun.NewAnalyzer("postgresql", nil)

	migration := &models.Migration{
		Operations: []*models.Operation{
			{Type: models.OperationTypeAddColumn, TableName: "users", SQL: "ALTER TABLE users ADD COLUMN email VARCHAR(255)"},
			{Type: models.OperationTypeCreateIndex, TableName: "users", SQL: "CREATE INDEX idx_email ON users(email)"},
		},
	}

	ctx := context.Background()
	err := analyzer.AnalyzeMigration(ctx, migration)
	if err != nil {
		t.Fatalf("AnalyzeMigration failed: %v", err)
	}

	// Verify both operations have results
	for i, op := range migration.Operations {
		if op.DryRunResult == nil {
			t.Errorf("Operation %d: DryRunResult should be populated", i)
		}
	}
}

func TestAnalyzer_EmptyMigration(t *testing.T) {
	analyzer := dryrun.NewAnalyzer("postgresql", nil)

	migration := &models.Migration{
		Operations: []*models.Operation{},
	}

	ctx := context.Background()
	err := analyzer.AnalyzeMigration(ctx, migration)
	if err != nil {
		t.Fatalf("AnalyzeMigration on empty migration failed: %v", err)
	}
}

func TestAnalyzer_NilOperationHandling(t *testing.T) {
	analyzer := dryrun.NewAnalyzer("postgresql", nil)

	migration := &models.Migration{
		Operations: []*models.Operation{
			nil, // This should not cause panic
			{Type: models.OperationTypeAddColumn, TableName: "users", SQL: "ALTER TABLE users ADD COLUMN email VARCHAR(255)"},
		},
	}

	ctx := context.Background()
	err := analyzer.AnalyzeMigration(ctx, migration)
	// Should handle nil gracefully (either skip or return error, but not panic)
	if err != nil {
		t.Fatalf("AnalyzeMigration with nil operation failed: %v", err)
	}

	// Verify non-nil operation has result
	if migration.Operations[1].DryRunResult == nil {
		t.Error("Non-nil operation should have DryRunResult populated")
	}
}
