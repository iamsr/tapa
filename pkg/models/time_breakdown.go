package models

import (
	"fmt"
	"strings"
)

// TimeBreakdown shows where time is spent in an operation
type TimeBreakdown struct {
	TableRewriteSeconds    float64 `json:"table_rewrite_seconds"`
	IndexBuildSeconds      float64 `json:"index_build_seconds"`
	ConstraintCheckSeconds float64 `json:"constraint_check_seconds"`
	MetadataUpdateSeconds  float64 `json:"metadata_update_seconds"`
	TotalSeconds           float64 `json:"total_seconds"`
}

// CalculateTotal sums all components
func (tb *TimeBreakdown) CalculateTotal() {
	tb.TotalSeconds = tb.TableRewriteSeconds +
		tb.IndexBuildSeconds +
		tb.ConstraintCheckSeconds +
		tb.MetadataUpdateSeconds
}

// String returns human-readable breakdown
func (tb *TimeBreakdown) String() string {
	parts := []string{}

	if tb.TableRewriteSeconds > 0 {
		parts = append(parts, fmt.Sprintf("Table Rewrite: %.1fs", tb.TableRewriteSeconds))
	}
	if tb.IndexBuildSeconds > 0 {
		parts = append(parts, fmt.Sprintf("Index Build: %.1fs", tb.IndexBuildSeconds))
	}
	if tb.ConstraintCheckSeconds > 0 {
		parts = append(parts, fmt.Sprintf("Constraint Check: %.1fs", tb.ConstraintCheckSeconds))
	}
	if tb.MetadataUpdateSeconds > 0 {
		parts = append(parts, fmt.Sprintf("Metadata: %.1fs", tb.MetadataUpdateSeconds))
	}

	parts = append(parts, fmt.Sprintf("Total: %.1fs", tb.TotalSeconds))

	return strings.Join(parts, ", ")
}
