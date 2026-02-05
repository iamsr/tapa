package models

import (
	"testing"
)

func TestMigrationBatch_MaxRisk(t *testing.T) {
	tests := []struct {
		name              string
		operations        []*Operation
		expectedMaxRisk   int
		expectedRiskLevel RiskLevel
	}{
		{
			name: "critical risk operations",
			operations: []*Operation{
				{RiskScore: 50},
				{RiskScore: 80},
				{RiskScore: 60},
			},
			expectedMaxRisk:   80,
			expectedRiskLevel: RiskLevelCritical,
		},
		{
			name: "high risk operations",
			operations: []*Operation{
				{RiskScore: 30},
				{RiskScore: 65},
				{RiskScore: 40},
			},
			expectedMaxRisk:   65,
			expectedRiskLevel: RiskLevelHigh,
		},
		{
			name: "medium risk operations",
			operations: []*Operation{
				{RiskScore: 10},
				{RiskScore: 35},
				{RiskScore: 20},
			},
			expectedMaxRisk:   35,
			expectedRiskLevel: RiskLevelMedium,
		},
		{
			name: "low risk operations",
			operations: []*Operation{
				{RiskScore: 5},
				{RiskScore: 15},
				{RiskScore: 20},
			},
			expectedMaxRisk:   20,
			expectedRiskLevel: RiskLevelLow,
		},
		{
			name: "boundary: risk score 76 is critical",
			operations: []*Operation{
				{RiskScore: 76},
			},
			expectedMaxRisk:   76,
			expectedRiskLevel: RiskLevelCritical,
		},
		{
			name: "boundary: risk score 75 is high",
			operations: []*Operation{
				{RiskScore: 75},
			},
			expectedMaxRisk:   75,
			expectedRiskLevel: RiskLevelHigh,
		},
		{
			name: "boundary: risk score 51 is high",
			operations: []*Operation{
				{RiskScore: 51},
			},
			expectedMaxRisk:   51,
			expectedRiskLevel: RiskLevelHigh,
		},
		{
			name: "boundary: risk score 50 is medium",
			operations: []*Operation{
				{RiskScore: 50},
			},
			expectedMaxRisk:   50,
			expectedRiskLevel: RiskLevelMedium,
		},
		{
			name: "boundary: risk score 26 is medium",
			operations: []*Operation{
				{RiskScore: 26},
			},
			expectedMaxRisk:   26,
			expectedRiskLevel: RiskLevelMedium,
		},
		{
			name: "boundary: risk score 25 is low",
			operations: []*Operation{
				{RiskScore: 25},
			},
			expectedMaxRisk:   25,
			expectedRiskLevel: RiskLevelLow,
		},
		{
			name: "single operation",
			operations: []*Operation{
				{RiskScore: 45},
			},
			expectedMaxRisk:   45,
			expectedRiskLevel: RiskLevelMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := &MigrationBatch{
				Operations: tt.operations,
			}

			batch.CalculateMetrics()

			if batch.MaxRiskScore != tt.expectedMaxRisk {
				t.Errorf("MaxRiskScore = %d, want %d", batch.MaxRiskScore, tt.expectedMaxRisk)
			}

			if batch.RiskLevel != tt.expectedRiskLevel {
				t.Errorf("RiskLevel = %s, want %s", batch.RiskLevel, tt.expectedRiskLevel)
			}
		})
	}
}

func TestMigrationBatch_TotalTime(t *testing.T) {
	tests := []struct {
		name              string
		operations        []*Operation
		expectedTotalTime float64
	}{
		{
			name: "multiple operations with time",
			operations: []*Operation{
				{EstimatedTimeSeconds: 10.5},
				{EstimatedTimeSeconds: 20.3},
				{EstimatedTimeSeconds: 5.2},
			},
			expectedTotalTime: 36.0,
		},
		{
			name: "single operation",
			operations: []*Operation{
				{EstimatedTimeSeconds: 15.5},
			},
			expectedTotalTime: 15.5,
		},
		{
			name: "operations with zero time",
			operations: []*Operation{
				{EstimatedTimeSeconds: 0},
				{EstimatedTimeSeconds: 10.0},
				{EstimatedTimeSeconds: 0},
			},
			expectedTotalTime: 10.0,
		},
		{
			name: "all zero time",
			operations: []*Operation{
				{EstimatedTimeSeconds: 0},
				{EstimatedTimeSeconds: 0},
			},
			expectedTotalTime: 0.0,
		},
		{
			name: "precise decimal addition",
			operations: []*Operation{
				{EstimatedTimeSeconds: 1.111},
				{EstimatedTimeSeconds: 2.222},
				{EstimatedTimeSeconds: 3.333},
			},
			expectedTotalTime: 6.666,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := &MigrationBatch{
				Operations: tt.operations,
			}

			batch.CalculateMetrics()

			// Use approximate comparison for floating point
			if diff := batch.TotalTimeSeconds - tt.expectedTotalTime; diff > 0.001 || diff < -0.001 {
				t.Errorf("TotalTimeSeconds = %f, want %f", batch.TotalTimeSeconds, tt.expectedTotalTime)
			}
		})
	}
}

func TestMigrationBatch_EdgeCases(t *testing.T) {
	t.Run("empty operations slice", func(t *testing.T) {
		batch := &MigrationBatch{
			Operations: []*Operation{},
		}

		batch.CalculateMetrics()

		if batch.MaxRiskScore != 0 {
			t.Errorf("MaxRiskScore = %d, want 0", batch.MaxRiskScore)
		}

		if batch.RiskLevel != RiskLevelLow {
			t.Errorf("RiskLevel = %s, want %s", batch.RiskLevel, RiskLevelLow)
		}

		if batch.TotalTimeSeconds != 0.0 {
			t.Errorf("TotalTimeSeconds = %f, want 0.0", batch.TotalTimeSeconds)
		}
	})

	t.Run("nil operations slice", func(t *testing.T) {
		batch := &MigrationBatch{
			Operations: nil,
		}

		batch.CalculateMetrics()

		if batch.MaxRiskScore != 0 {
			t.Errorf("MaxRiskScore = %d, want 0", batch.MaxRiskScore)
		}

		if batch.RiskLevel != RiskLevelLow {
			t.Errorf("RiskLevel = %s, want %s", batch.RiskLevel, RiskLevelLow)
		}

		if batch.TotalTimeSeconds != 0.0 {
			t.Errorf("TotalTimeSeconds = %f, want 0.0", batch.TotalTimeSeconds)
		}
	})

	t.Run("combined risk and time calculation", func(t *testing.T) {
		batch := &MigrationBatch{
			Operations: []*Operation{
				{RiskScore: 30, EstimatedTimeSeconds: 10.0},
				{RiskScore: 80, EstimatedTimeSeconds: 20.5},
				{RiskScore: 45, EstimatedTimeSeconds: 5.5},
			},
		}

		batch.CalculateMetrics()

		if batch.MaxRiskScore != 80 {
			t.Errorf("MaxRiskScore = %d, want 80", batch.MaxRiskScore)
		}

		if batch.RiskLevel != RiskLevelCritical {
			t.Errorf("RiskLevel = %s, want %s", batch.RiskLevel, RiskLevelCritical)
		}

		expectedTime := 36.0
		if diff := batch.TotalTimeSeconds - expectedTime; diff > 0.001 || diff < -0.001 {
			t.Errorf("TotalTimeSeconds = %f, want %f", batch.TotalTimeSeconds, expectedTime)
		}
	})
}

func TestBatchingStrategy_CalculateMetrics(t *testing.T) {
	tests := []struct {
		name               string
		batches            []MigrationBatch
		expectedTotalBatch int
	}{
		{
			name: "multiple batches",
			batches: []MigrationBatch{
				{Operations: []*Operation{{RiskScore: 30}}},
				{Operations: []*Operation{{RiskScore: 50}}},
				{Operations: []*Operation{{RiskScore: 80}}},
			},
			expectedTotalBatch: 3,
		},
		{
			name: "single batch",
			batches: []MigrationBatch{
				{Operations: []*Operation{{RiskScore: 30}}},
			},
			expectedTotalBatch: 1,
		},
		{
			name:               "empty batches",
			batches:            []MigrationBatch{},
			expectedTotalBatch: 0,
		},
		{
			name:               "nil batches",
			batches:            nil,
			expectedTotalBatch: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &BatchingStrategy{
				Batches: tt.batches,
			}

			strategy.CalculateMetrics()

			if strategy.TotalBatches != tt.expectedTotalBatch {
				t.Errorf("TotalBatches = %d, want %d", strategy.TotalBatches, tt.expectedTotalBatch)
			}
		})
	}
}

func TestBatchingStrategy_CalculatesEachBatchMetrics(t *testing.T) {
	t.Run("calculates metrics for each batch", func(t *testing.T) {
		strategy := &BatchingStrategy{
			Batches: []MigrationBatch{
				{
					Operations: []*Operation{
						{RiskScore: 30, EstimatedTimeSeconds: 10.0},
						{RiskScore: 60, EstimatedTimeSeconds: 15.0},
					},
				},
				{
					Operations: []*Operation{
						{RiskScore: 80, EstimatedTimeSeconds: 20.0},
					},
				},
			},
		}

		strategy.CalculateMetrics()

		// First batch should have max risk 60 and total time 25.0
		if strategy.Batches[0].MaxRiskScore != 60 {
			t.Errorf("Batch[0].MaxRiskScore = %d, want 60", strategy.Batches[0].MaxRiskScore)
		}
		if strategy.Batches[0].RiskLevel != RiskLevelHigh {
			t.Errorf("Batch[0].RiskLevel = %s, want %s", strategy.Batches[0].RiskLevel, RiskLevelHigh)
		}
		expectedTime := 25.0
		if diff := strategy.Batches[0].TotalTimeSeconds - expectedTime; diff > 0.001 || diff < -0.001 {
			t.Errorf("Batch[0].TotalTimeSeconds = %f, want %f", strategy.Batches[0].TotalTimeSeconds, expectedTime)
		}

		// Second batch should have max risk 80 and total time 20.0
		if strategy.Batches[1].MaxRiskScore != 80 {
			t.Errorf("Batch[1].MaxRiskScore = %d, want 80", strategy.Batches[1].MaxRiskScore)
		}
		if strategy.Batches[1].RiskLevel != RiskLevelCritical {
			t.Errorf("Batch[1].RiskLevel = %s, want %s", strategy.Batches[1].RiskLevel, RiskLevelCritical)
		}
		expectedTime = 20.0
		if diff := strategy.Batches[1].TotalTimeSeconds - expectedTime; diff > 0.001 || diff < -0.001 {
			t.Errorf("Batch[1].TotalTimeSeconds = %f, want %f", strategy.Batches[1].TotalTimeSeconds, expectedTime)
		}

		// Total batches should be 2
		if strategy.TotalBatches != 2 {
			t.Errorf("TotalBatches = %d, want 2", strategy.TotalBatches)
		}
	})
}
