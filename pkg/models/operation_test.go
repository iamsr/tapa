package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestOperation_IsHighRisk(t *testing.T) {
	tests := []struct {
		name      string
		riskScore int
		expected  bool
	}{
		{"low risk", 25, false},
		{"medium risk", 40, false},
		{"high risk boundary", 51, true},
		{"high risk", 75, true},
		{"critical risk", 90, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &Operation{RiskScore: tt.riskScore}
			assert.Equal(t, tt.expected, op.IsHighRisk())
		})
	}
}

func TestOperation_RiskLevel(t *testing.T) {
	tests := []struct {
		riskScore int
		expected  RiskLevel
	}{
		{10, RiskLevelLow},
		{25, RiskLevelLow},
		{30, RiskLevelMedium},
		{50, RiskLevelMedium},
		{55, RiskLevelHigh},
		{75, RiskLevelHigh},
		{80, RiskLevelCritical},
		{95, RiskLevelCritical},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			op := &Operation{RiskScore: tt.riskScore}
			assert.Equal(t, tt.expected, op.RiskLevel())
		})
	}
}

func TestOperation_EstimatedDuration(t *testing.T) {
	op := &Operation{
		EstimatedTimeSeconds: 125.5,
	}

	duration := op.EstimatedDuration()
	assert.Equal(t, 125*time.Second+500*time.Millisecond, duration)
}

func TestOperation_HasBreakingDependencies(t *testing.T) {
	op := &Operation{
		Type:      OperationTypeDropColumn,
		TableName: "users",
		Dependencies: []Dependency{
			{
				Type:        DependencyTypeIndex,
				Name:        "idx_users_email",
				ImpactLevel: ImpactBreaks,
			},
		},
	}

	hasBreaking := false
	for _, dep := range op.Dependencies {
		if dep.IsBreaking() {
			hasBreaking = true
			break
		}
	}

	assert.True(t, hasBreaking)
}
