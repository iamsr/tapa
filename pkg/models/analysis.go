package models

// AnalysisResult contains the complete analysis output
type AnalysisResult struct {
	Migrations      []*Migration
	DatabaseType    string
	FailOnRiskLevel RiskLevel
	Errors          []error
}

// MaxRiskScore returns the highest risk score across all migrations
func (a *AnalysisResult) MaxRiskScore() int {
	max := 0
	for _, m := range a.Migrations {
		score := m.MaxRiskScore()
		if score > max {
			max = score
		}
	}
	return max
}

// HasFailures returns true if any operation exceeds the fail threshold
func (a *AnalysisResult) HasFailures() bool {
	threshold := riskLevelToScore(a.FailOnRiskLevel)
	return a.MaxRiskScore() >= threshold
}

// riskLevelToScore converts risk level to numeric threshold
func riskLevelToScore(level RiskLevel) int {
	switch level {
	case RiskLevelCritical:
		return 76
	case RiskLevelHigh:
		return 51
	case RiskLevelMedium:
		return 26
	case RiskLevelLow:
		return 1
	default:
		return 101 // Never fail
	}
}
