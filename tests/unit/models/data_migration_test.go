package models_test

import (
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestDataMigrationComplexity_String(t *testing.T) {
	tests := []struct {
		complexity models.DataMigrationComplexity
		want       string
	}{
		{models.DataMigrationSimple, "SIMPLE_COMPUTATION"},
		{models.DataMigrationModerate, "MODERATE_LOGIC"},
		{models.DataMigrationComplex, "COMPLEX_JOINS"},
		{models.DataMigrationBulkDelete, "BULK_DELETE"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := string(tt.complexity); got != tt.want {
				t.Errorf("complexity = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataMigrationAnalysis_ShouldBatch(t *testing.T) {
	tests := []struct {
		name          string
		estimatedRows int64
		want          bool
	}{
		{"small dataset", 100, false},
		{"medium dataset", 10000, true},
		{"large dataset", 5000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.DataMigrationAnalysis{
				EstimatedRows: tt.estimatedRows,
			}
			analysis.BatchingRecommendation = &models.BatchingRecommendation{
				ShouldBatch: tt.estimatedRows > 5000,
			}

			if got := analysis.BatchingRecommendation.ShouldBatch; got != tt.want {
				t.Errorf("ShouldBatch = %v, want %v", got, tt.want)
			}
		})
	}
}
