package models_test

import (
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestConcurrencyAnalysis_IsHighImpact(t *testing.T) {
	tests := []struct {
		name        string
		impactScore int
		want        bool
	}{
		{"low impact", 20, false},
		{"medium impact", 45, false},
		{"high impact", 70, true},
		{"critical impact", 95, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.ConcurrencyAnalysis{
				ImpactScore: tt.impactScore,
			}
			if got := analysis.IsHighImpact(); got != tt.want {
				t.Errorf("IsHighImpact() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcurrencyAnalysis_ImpactLevel(t *testing.T) {
	tests := []struct {
		name        string
		impactScore int
		want        models.ConcurrencyImpactLevel
	}{
		{"minimal", 10, models.ConcurrencyImpactMinimal},
		{"low", 30, models.ConcurrencyImpactLow},
		{"medium", 55, models.ConcurrencyImpactMedium},
		{"high", 75, models.ConcurrencyImpactHigh},
		{"critical", 95, models.ConcurrencyImpactCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.ConcurrencyAnalysis{
				ImpactScore: tt.impactScore,
			}
			if got := analysis.ImpactLevel(); got != tt.want {
				t.Errorf("ImpactLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLockImpact_String(t *testing.T) {
	impact := &models.LockImpact{
		LockType:              models.LockTypeAccessExclusive,
		EstimatedDurationMS:   5000,
		BlockedQueryTypes:     []string{"SELECT", "INSERT", "UPDATE"},
		EstimatedBlockedCount: 25,
		WaitTimeRange:         "2-5 seconds",
	}

	str := impact.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
}
