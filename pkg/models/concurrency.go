package models

import "fmt"

// ConcurrencyImpactLevel represents the severity of concurrency impact
type ConcurrencyImpactLevel string

const (
	ConcurrencyImpactMinimal  ConcurrencyImpactLevel = "MINIMAL"  // 0-20
	ConcurrencyImpactLow      ConcurrencyImpactLevel = "LOW"      // 21-40
	ConcurrencyImpactMedium   ConcurrencyImpactLevel = "MEDIUM"   // 41-60
	ConcurrencyImpactHigh     ConcurrencyImpactLevel = "HIGH"     // 61-80
	ConcurrencyImpactCritical ConcurrencyImpactLevel = "CRITICAL" // 81-100
)

// ConcurrencyAnalysis represents concurrency impact analysis for a migration
type ConcurrencyAnalysis struct {
	ImpactScore         int                     `json:"impact_score"` // 0-100
	LockImpact          *LockImpact             `json:"lock_impact"`
	WorkloadAnalysis    *WorkloadAnalysis       `json:"workload_analysis,omitempty"`
	SaferAlternatives   []ConcurrentAlternative `json:"safer_alternatives,omitempty"`
	Recommendations     []string                `json:"recommendations"`
	EstimatedDowntimeMS int64                   `json:"estimated_downtime_ms,omitempty"`
	ConcurrencySafe     bool                    `json:"concurrency_safe"`
}

// LockImpact describes the locking behavior and impact
type LockImpact struct {
	LockType              LockType `json:"lock_type"`
	EstimatedDurationMS   int64    `json:"estimated_duration_ms"`
	BlocksReads           bool     `json:"blocks_reads"`
	BlocksWrites          bool     `json:"blocks_writes"`
	BlockedQueryTypes     []string `json:"blocked_query_types"`     // ["SELECT", "INSERT", "UPDATE", "DELETE"]
	EstimatedBlockedCount int      `json:"estimated_blocked_count"` // Number of queries expected to be blocked
	WaitTimeRange         string   `json:"wait_time_range"`         // "0-2 seconds", "5-30 seconds"
	LockAcquisitionRisk   string   `json:"lock_acquisition_risk"`   // "low", "medium", "high"
}

// WorkloadAnalysis describes current database workload patterns
type WorkloadAnalysis struct {
	ActiveConnections    int                `json:"active_connections"`
	QueriesPerSecond     float64            `json:"queries_per_second"`
	TableAccessFrequency string             `json:"table_access_frequency"` // "low", "medium", "high", "very_high"
	PeakLoadPeriod       bool               `json:"peak_load_period"`
	TopQueryTypes        []QueryTypeMetrics `json:"top_query_types"`
	LongRunningQueries   int                `json:"long_running_queries"` // Count of queries > 5 seconds
}

// QueryTypeMetrics describes frequency of different query types
type QueryTypeMetrics struct {
	QueryType     string  `json:"query_type"` // "SELECT", "INSERT", "UPDATE", "DELETE"
	CountPerMin   int     `json:"count_per_min"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
	WillBeBlocked bool    `json:"will_be_blocked"`
}

// ConcurrentAlternative suggests safer concurrency approaches
type ConcurrentAlternative struct {
	Description     string   `json:"description"`
	LockType        LockType `json:"lock_type"`
	ImpactReduction int      `json:"impact_reduction"`           // Percentage reduction in impact score
	RequiresFeature string   `json:"requires_feature,omitempty"` // e.g., "PostgreSQL 11+", "ALGORITHM=INPLACE"
	Steps           []string `json:"steps,omitempty"`
	EstimatedTimeMS int64    `json:"estimated_time_ms"`
	Tradeoffs       []string `json:"tradeoffs,omitempty"`
}

// IsHighImpact returns true if impact score >= 61
func (c *ConcurrencyAnalysis) IsHighImpact() bool {
	return c.ImpactScore >= 61
}

// ImpactLevel returns the impact category based on score
func (c *ConcurrencyAnalysis) ImpactLevel() ConcurrencyImpactLevel {
	switch {
	case c.ImpactScore >= 81:
		return ConcurrencyImpactCritical
	case c.ImpactScore >= 61:
		return ConcurrencyImpactHigh
	case c.ImpactScore >= 41:
		return ConcurrencyImpactMedium
	case c.ImpactScore >= 21:
		return ConcurrencyImpactLow
	default:
		return ConcurrencyImpactMinimal
	}
}

// String returns a string representation of lock impact
func (l *LockImpact) String() string {
	return fmt.Sprintf("%s lock for %dms, blocks %d queries", l.LockType, l.EstimatedDurationMS, l.EstimatedBlockedCount)
}

// BlocksAllWrites returns true if lock blocks all write operations
func (l *LockImpact) BlocksAllWrites() bool {
	return l.LockType == LockTypeAccessExclusive || l.LockType == LockTypeExclusive
}

// BlocksAllReads returns true if lock blocks all read operations
func (l *LockImpact) BlocksAllReads() bool {
	return l.LockType == LockTypeAccessExclusive
}
