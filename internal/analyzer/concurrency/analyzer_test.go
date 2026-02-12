package concurrency_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAnalyzer_AnalyzeOperation(t *testing.T) {
	analyzer := concurrency.NewAnalyzer("postgresql", nil)

	tests := []struct {
		name             string
		op               *models.Operation
		expectHighImpact bool
	}{
		{
			name: "ACCESS EXCLUSIVE on large table",
			op: &models.Operation{
				Type:                 models.OperationTypeAddColumn,
				TableName:            "users",
				RequiresRewrite:      true,
				LockType:             models.LockTypeAccessExclusive,
				RowCount:             5000000,
				EstimatedTimeSeconds: 120.0,
			},
			expectHighImpact: true,
		},
		{
			name: "CREATE INDEX CONCURRENTLY low impact",
			op: &models.Operation{
				Type:            models.OperationTypeCreateIndex,
				TableName:       "orders",
				RequiresRewrite: false,
				LockType:        models.LockTypeShareUpdateExclusive,
				RowCount:        1000000,
			},
			expectHighImpact: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzer.AnalyzeOperation(ctx, tt.op)
			if err != nil {
				t.Fatalf("AnalyzeOperation failed: %v", err)
			}

			if tt.op.ConcurrencyAnalysis == nil {
				t.Fatal("ConcurrencyAnalysis should be populated")
			}

			if got := tt.op.ConcurrencyAnalysis.IsHighImpact(); got != tt.expectHighImpact {
				t.Errorf("IsHighImpact() = %v, want %v (score: %d)", got, tt.expectHighImpact, tt.op.ConcurrencyAnalysis.ImpactScore)
			}
		})
	}
}

func TestAnalyzer_GenerateAlternatives(t *testing.T) {
	analyzer := concurrency.NewAnalyzer("postgresql", nil)

	op := &models.Operation{
		Type:                 models.OperationTypeCreateIndex,
		TableName:            "users",
		IndexName:            "idx_users_email",
		LockType:             models.LockTypeShare,
		RowCount:             1000000,
		EstimatedTimeSeconds: 60.0,
	}

	alternatives := analyzer.GenerateAlternatives(op)

	// Should suggest CONCURRENTLY for PostgreSQL
	if len(alternatives) == 0 {
		t.Error("Expected at least one alternative")
	}

	foundConcurrent := false
	for _, alt := range alternatives {
		if alt.LockType == models.LockTypeShareUpdateExclusive {
			foundConcurrent = true
			break
		}
	}

	if !foundConcurrent {
		t.Error("Expected CONCURRENT alternative for CREATE INDEX")
	}
}
