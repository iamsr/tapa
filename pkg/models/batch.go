package models

// MigrationBatch represents a group of operations batched together
type MigrationBatch struct {
	BatchNumber      int          `json:"batch_number"`
	Operations       []*Operation `json:"operations"`
	MaxRiskScore     int          `json:"max_risk_score"`
	RiskLevel        RiskLevel    `json:"risk_level"`
	TotalTimeSeconds float64      `json:"total_time_seconds"`
	CanRunInParallel bool         `json:"can_run_in_parallel"`
	Prerequisites    []int        `json:"prerequisites,omitempty"`
	Rationale        string       `json:"rationale"`
}

// CalculateMetrics computes max risk score, risk level, and total time from operations
func (mb *MigrationBatch) CalculateMetrics() {
	// Reset values
	mb.MaxRiskScore = 0
	mb.TotalTimeSeconds = 0

	// Calculate max risk and total time
	for _, op := range mb.Operations {
		if op.RiskScore > mb.MaxRiskScore {
			mb.MaxRiskScore = op.RiskScore
		}
		mb.TotalTimeSeconds += op.EstimatedTimeSeconds
	}

	// Set risk level based on max risk score (same thresholds as Operation)
	switch {
	case mb.MaxRiskScore >= 76:
		mb.RiskLevel = RiskLevelCritical
	case mb.MaxRiskScore >= 51:
		mb.RiskLevel = RiskLevelHigh
	case mb.MaxRiskScore >= 26:
		mb.RiskLevel = RiskLevelMedium
	default:
		mb.RiskLevel = RiskLevelLow
	}
}

// BatchingStrategy represents a complete batching plan for a migration
type BatchingStrategy struct {
	OriginalMigration string           `json:"original_migration"`
	Batches           []MigrationBatch `json:"batches"`
	TotalBatches      int              `json:"total_batches"`
	TotalOperations   int              `json:"total_operations"`
	TotalTimeSeconds  float64          `json:"total_time_seconds"`
	MaxRiskLevel      RiskLevel        `json:"max_risk_level"`
	Recommendations   []string         `json:"recommendations"`
}

// CalculateMetrics computes total batches and calculates metrics for each batch
func (bs *BatchingStrategy) CalculateMetrics() {
	bs.TotalBatches = len(bs.Batches)
	bs.TotalOperations = 0
	bs.TotalTimeSeconds = 0
	bs.MaxRiskLevel = RiskLevelLow

	// Calculate metrics for each batch
	for i := range bs.Batches {
		bs.Batches[i].CalculateMetrics()

		// Aggregate totals
		bs.TotalOperations += len(bs.Batches[i].Operations)
		bs.TotalTimeSeconds += bs.Batches[i].TotalTimeSeconds

		// Track max risk level
		if bs.Batches[i].RiskLevel > bs.MaxRiskLevel {
			bs.MaxRiskLevel = bs.Batches[i].RiskLevel
		}
	}
}
