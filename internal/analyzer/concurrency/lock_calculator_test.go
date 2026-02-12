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

func TestLockCalculator_CalculateImpact_NilOperation(t *testing.T) {
	calculator := concurrency.NewLockCalculator("postgresql")

	impact := calculator.CalculateImpact(nil)

	if impact != nil {
		t.Error("Expected nil impact for nil operation, got non-nil")
	}
}

func TestLockCalculator_EstimateDuration_OperationTypes(t *testing.T) {
	calculator := concurrency.NewLockCalculator("postgresql")

	tests := []struct {
		name        string
		op          *models.Operation
		wantMinimum int64
	}{
		{
			name: "CREATE INDEX takes longer",
			op: &models.Operation{
				Type:     models.OperationTypeCreateIndex,
				RowCount: 100000,
			},
			wantMinimum: 500, // Should be significantly higher than base
		},
		{
			name: "DROP COLUMN is fast",
			op: &models.Operation{
				Type: models.OperationTypeDropColumn,
			},
			wantMinimum: 50, // Should be at minimum
		},
		{
			name: "Zero row count",
			op: &models.Operation{
				Type:     models.OperationTypeAddColumn,
				RowCount: 0,
			},
			wantMinimum: 100, // Should use minimum
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := calculator.EstimateLockDuration(tt.op)
			if duration < tt.wantMinimum {
				t.Errorf("Duration %d ms is less than minimum %d ms", duration, tt.wantMinimum)
			}
		})
	}
}

func TestLockCalculator_CalculateWaitTimeRange(t *testing.T) {
	calculator := concurrency.NewLockCalculator("postgresql")

	tests := []struct {
		name       string
		durationMS int64
		wantRange  string
	}{
		{"under 1 second", 500, "< 1 second"},
		{"1-2 seconds", 1500, "1-2 seconds"},
		{"2-5 seconds", 3000, "2-5 seconds"},
		{"5-10 seconds", 7000, "5-10 seconds"},
		{"10-30 seconds", 20000, "10-30 seconds"},
		{"30-60 seconds", 45000, "30-60 seconds"},
		{"1-5 minutes", 180000, "1-5 minutes"},
		{"5-10 minutes", 400000, "5-10 minutes"},
		{"over 10 minutes", 700000, "> 10 minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test indirectly through CalculateImpact since calculateWaitTimeRange is private
			op := &models.Operation{
				Type:                 models.OperationTypeAddColumn,
				LockType:             models.LockTypeAccessExclusive,
				RowCount:             1000000,
				EstimatedTimeSeconds: float64(tt.durationMS) / 1000.0,
			}
			impact := calculator.CalculateImpact(op)
			if impact.WaitTimeRange != tt.wantRange {
				t.Errorf("WaitTimeRange for %d ms = %q, want %q", tt.durationMS, impact.WaitTimeRange, tt.wantRange)
			}
		})
	}
}
