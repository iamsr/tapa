package concurrency_test

import (
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/pkg/models"
)

func TestLockCalculator_CalculateImpact(t *testing.T) {
	calculator := concurrency.NewLockCalculator("postgresql")

	tests := []struct {
		name          string
		op            *models.Operation
		wantBlocksAll bool
	}{
		{
			name: "ADD COLUMN with default blocks all",
			op: &models.Operation{
				Type:                 models.OperationTypeAddColumn,
				TableName:            "users",
				RequiresRewrite:      true,
				LockType:             models.LockTypeAccessExclusive,
				RowCount:             1000000,
				EstimatedTimeSeconds: 0,
			},
			wantBlocksAll: true,
		},
		{
			name: "CREATE INDEX CONCURRENTLY low impact",
			op: &models.Operation{
				Type:                 models.OperationTypeCreateIndex,
				TableName:            "orders",
				RequiresRewrite:      false,
				LockType:             models.LockTypeShareUpdateExclusive,
				RowCount:             500000,
				EstimatedTimeSeconds: 0,
			},
			wantBlocksAll: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impact := calculator.CalculateImpact(tt.op)

			if impact == nil {
				t.Fatal("Expected lock impact, got nil")
			}

			if got := impact.BlocksReads; got != tt.wantBlocksAll {
				t.Errorf("BlocksReads = %v, want %v", got, tt.wantBlocksAll)
			}
		})
	}
}

func TestLockCalculator_EstimateDuration(t *testing.T) {
	calculator := concurrency.NewLockCalculator("postgresql")

	op := &models.Operation{
		Type:                 models.OperationTypeAddColumn,
		RequiresRewrite:      true,
		RowCount:             1000000,
		EstimatedTimeSeconds: 30.0,
	}

	duration := calculator.EstimateLockDuration(op)

	if duration <= 0 {
		t.Error("Lock duration should be positive")
	}

	// Lock duration should be based on estimated time
	expectedMS := int64(30000) // 30 seconds
	tolerance := int64(5000)   // 5 second tolerance

	if duration < expectedMS-tolerance || duration > expectedMS+tolerance {
		t.Errorf("Lock duration %d ms not within expected range %d±%d ms", duration, expectedMS, tolerance)
	}
}
