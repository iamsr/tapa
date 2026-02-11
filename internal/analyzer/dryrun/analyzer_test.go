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
