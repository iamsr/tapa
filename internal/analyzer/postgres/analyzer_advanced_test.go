package postgres_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/postgres"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAnalyzer_ComprehensiveAnalysis(t *testing.T) {
	// Create analyzer with comprehensive mode
	analyzer := postgres.NewAnalyzer(nil, 200, 2.0, true)

	op := &models.Operation{
		Type:            models.OperationTypeAlterColumn,
		TableName:       "users",
		SQL:             "ALTER TABLE users ALTER COLUMN email TYPE TEXT; UPDATE users SET email = lower(email);",
		RequiresRewrite: true,
	}

	// Note: We're not using stats directly, but the analyzer will use default stats
	// since we passed nil introspector. The test is to verify comprehensive mode
	// triggers all 3 advanced analyzers.

	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Verify advanced features are populated
	if op.DiskSpaceAnalysis == nil {
		t.Error("DiskSpaceAnalysis should be populated in comprehensive mode")
	}

	if op.RollbackAnalysis == nil {
		t.Error("RollbackAnalysis should be populated in comprehensive mode")
	}

	if op.DataMigrationAnalysis == nil {
		t.Error("DataMigrationAnalysis should be populated when UPDATE detected")
	}
}

const GB = 1024 * 1024 * 1024
