package models

import "fmt"

// DiskSpaceAnalysis represents disk space requirements for a migration
type DiskSpaceAnalysis struct {
	CurrentState          DiskSpaceState            `json:"current_state"`
	MigrationRequirements MigrationDiskRequirements `json:"migration_requirements"`
	FinalState            DiskSpaceState            `json:"final_state"`
	SystemCheck           SystemDiskCheck           `json:"system_check"`
}

// DiskSpaceState represents disk space at a point in time
type DiskSpaceState struct {
	TableSizeBytes int64 `json:"table_size_bytes"`
	IndexSizeBytes int64 `json:"index_size_bytes"`
	ToastSizeBytes int64 `json:"toast_size_bytes,omitempty"` // PostgreSQL only
	DeadTupleBytes int64 `json:"dead_tuple_bytes,omitempty"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
}

// MigrationDiskRequirements represents space needed during migration
type MigrationDiskRequirements struct {
	RequiresRewrite     bool  `json:"requires_rewrite"`
	TemporaryTableBytes int64 `json:"temporary_table_bytes"`
	NewIndexBytes       int64 `json:"new_index_bytes,omitempty"`
	TransactionLogBytes int64 `json:"transaction_log_bytes,omitempty"`
	PeakDiskUsageBytes  int64 `json:"peak_disk_usage_bytes"`
	SafetyBufferBytes   int64 `json:"safety_buffer_bytes"`
}

// SystemDiskCheck represents system-level disk availability
type SystemDiskCheck struct {
	AvailableBytes     int64   `json:"available_bytes"`
	RequiredBytes      int64   `json:"required_bytes"`
	HasSufficientSpace bool    `json:"has_sufficient_space"`
	ShortfallBytes     int64   `json:"shortfall_bytes,omitempty"`
	WarningThreshold   float64 `json:"warning_threshold"` // e.g., 0.8 for 80%
}

// DatabaseBehavior represents database-specific space requirements
type DatabaseBehavior struct {
	DatabaseType        string  `json:"database_type"`
	RewriteMultiplier   float64 `json:"rewrite_multiplier"`   // e.g., 2.0 for PostgreSQL
	IndexBuildOverhead  float64 `json:"index_build_overhead"` // e.g., 1.2
	VacuumSpaceRequired bool    `json:"vacuum_space_required"`
}

// String returns human-readable representation
func (d *DiskSpaceAnalysis) String() string {
	status := "SUFFICIENT"
	if !d.SystemCheck.HasSufficientSpace {
		status = "INSUFFICIENT"
	}
	return fmt.Sprintf("DiskSpaceAnalysis{Current: %d GB, Peak: %d GB, Available: %d GB, Status: %s}",
		d.CurrentState.TotalSizeBytes/(1024*1024*1024),
		d.MigrationRequirements.PeakDiskUsageBytes/(1024*1024*1024),
		d.SystemCheck.AvailableBytes/(1024*1024*1024),
		status)
}
