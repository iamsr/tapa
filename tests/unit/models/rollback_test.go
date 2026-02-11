package models_test

import (
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestReversibilityCategory_String(t *testing.T) {
	tests := []struct {
		category models.ReversibilityCategory
		want     string
	}{
		{models.ReversibilitySafe, "SAFE"},
		{models.ReversibilityConditional, "CONDITIONAL"},
		{models.ReversibilityDataLoss, "DATA LOSS"},
		{models.ReversibilityIrreversible, "IRREVERSIBLE"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := string(tt.category); got != tt.want {
				t.Errorf("category = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollbackAnalysis_CanRollback(t *testing.T) {
	tests := []struct {
		name     string
		category models.ReversibilityCategory
		want     bool
	}{
		{"safe", models.ReversibilitySafe, true},
		{"conditional", models.ReversibilityConditional, true},
		{"data loss", models.ReversibilityDataLoss, false},
		{"irreversible", models.ReversibilityIrreversible, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.RollbackAnalysis{
				Category: tt.category,
			}
			if got := analysis.CanRollback(); got != tt.want {
				t.Errorf("CanRollback() = %v, want %v", got, tt.want)
			}
		})
	}
}
