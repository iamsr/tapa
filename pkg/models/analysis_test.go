package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAnalysisResult_TotalRiskScore(t *testing.T) {
	result := &AnalysisResult{
		Migrations: []*Migration{
			{Operations: []*Operation{{RiskScore: 30}, {RiskScore: 50}}},
			{Operations: []*Operation{{RiskScore: 70}}},
		},
	}

	assert.Equal(t, 70, result.MaxRiskScore())
}

func TestAnalysisResult_HasFailures(t *testing.T) {
	result := &AnalysisResult{
		FailOnRiskLevel: RiskLevelHigh,
		Migrations: []*Migration{
			{Operations: []*Operation{{RiskScore: 75}}},
		},
	}

	assert.True(t, result.HasFailures())

	result2 := &AnalysisResult{
		FailOnRiskLevel: RiskLevelHigh,
		Migrations: []*Migration{
			{Operations: []*Operation{{RiskScore: 40}}},
		},
	}

	assert.False(t, result2.HasFailures())
}
