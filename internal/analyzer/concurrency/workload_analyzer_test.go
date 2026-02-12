package concurrency_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/pkg/models"
)

func TestWorkloadAnalyzer_AnalyzeWorkload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	analyzer := concurrency.NewWorkloadAnalyzer("postgresql", nil)

	ctx := context.Background()
	workload, err := analyzer.AnalyzeWorkload(ctx, "users")

	// In mock mode (nil DB), should return defaults or skip
	if err != nil {
		t.Fatalf("AnalyzeWorkload failed: %v", err)
	}

	if workload != nil {
		if workload.ActiveConnections < 0 {
			t.Error("ActiveConnections should not be negative")
		}

		if workload.QueriesPerSecond < 0 {
			t.Error("QueriesPerSecond should not be negative")
		}
	}
}

func TestWorkloadAnalyzer_ClassifyAccessFrequency(t *testing.T) {
	analyzer := concurrency.NewWorkloadAnalyzer("postgresql", nil)

	tests := []struct {
		name          string
		queriesPerMin int
		expectedLevel string
	}{
		{"low traffic", 5, "low"},
		{"medium traffic", 150, "medium"},
		{"high traffic", 800, "high"},
		{"very high traffic", 2000, "very_high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frequency := analyzer.ClassifyAccessFrequency(tt.queriesPerMin)
			if frequency != tt.expectedLevel {
				t.Errorf("ClassifyAccessFrequency(%d) = %s, want %s", tt.queriesPerMin, frequency, tt.expectedLevel)
			}
		})
	}
}

func TestWorkloadAnalyzer_EstimateBlockedQueries(t *testing.T) {
	analyzer := concurrency.NewWorkloadAnalyzer("postgresql", nil)

	tests := []struct {
		name         string
		workload     *models.WorkloadAnalysis
		durationMS   int64
		blockedTypes []string
		wantMin      int
		wantMax      int
	}{
		{
			name: "With TopQueryTypes data",
			workload: &models.WorkloadAnalysis{
				TopQueryTypes: []models.QueryTypeMetrics{
					{QueryType: "SELECT", CountPerMin: 100},
					{QueryType: "UPDATE", CountPerMin: 50},
				},
			},
			durationMS:   60000, // 1 minute
			blockedTypes: []string{"SELECT", "UPDATE"},
			wantMin:      140, // Should be close to 150 (100 + 50)
			wantMax:      160,
		},
		{
			name:         "Nil workload",
			workload:     nil,
			durationMS:   60000,
			blockedTypes: []string{"SELECT"},
			wantMin:      0,
			wantMax:      0,
		},
		{
			name: "High frequency fallback",
			workload: &models.WorkloadAnalysis{
				QueriesPerSecond:     10.0,
				TableAccessFrequency: "high",
				PeakLoadPeriod:       false,
				TopQueryTypes:        []models.QueryTypeMetrics{}, // Empty, triggers fallback
			},
			durationMS:   30000, // 30 seconds
			blockedTypes: []string{"UPDATE"},
			wantMin:      100, // Should estimate based on frequency
			wantMax:      200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzer.EstimateBlockedQueries(tt.workload, tt.durationMS, tt.blockedTypes)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("EstimateBlockedQueries() = %d, want %d-%d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
