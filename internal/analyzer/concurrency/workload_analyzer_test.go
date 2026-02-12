package concurrency_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
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
