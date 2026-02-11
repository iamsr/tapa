package models

// DataMigrationComplexity represents the complexity of a data migration
type DataMigrationComplexity string

const (
	DataMigrationSimple     DataMigrationComplexity = "SIMPLE_COMPUTATION"
	DataMigrationModerate   DataMigrationComplexity = "MODERATE_LOGIC"
	DataMigrationComplex    DataMigrationComplexity = "COMPLEX_JOINS"
	DataMigrationBulkDelete DataMigrationComplexity = "BULK_DELETE"
)

// DataMigrationAnalysis represents analysis of data transformation operations
type DataMigrationAnalysis struct {
	HasDataMigration       bool                    `json:"has_data_migration"`
	OperationType          string                  `json:"operation_type"` // "UPDATE", "INSERT...SELECT", "DELETE"
	Complexity             DataMigrationComplexity `json:"complexity"`
	EstimatedRows          int64                   `json:"estimated_rows"`
	PerformanceEstimate    *PerformanceEstimate    `json:"performance_estimate"`
	BatchingRecommendation *BatchingRecommendation `json:"batching_recommendation,omitempty"`
	TableBloatImpact       *TableBloatImpact       `json:"table_bloat_impact,omitempty"`
}

// PerformanceEstimate represents estimated performance for data migration
type PerformanceEstimate struct {
	BaseSpeedRowsPerSecond     int     `json:"base_speed_rows_per_second"`
	AdjustedSpeedRowsPerSecond int     `json:"adjusted_speed_rows_per_second"` // Adjusted for indexes, complexity
	EstimatedDurationSeconds   float64 `json:"estimated_duration_seconds"`
	EstimatedDurationRange     string  `json:"estimated_duration_range"` // "10-12 minutes"
}

// BatchingRecommendation provides batching strategy for large data migrations
type BatchingRecommendation struct {
	ShouldBatch          bool   `json:"should_batch"`
	RecommendedBatchSize int    `json:"recommended_batch_size"`
	TotalBatches         int    `json:"total_batches"`
	PauseDurationMS      int    `json:"pause_duration_ms"`
	AllowsCancellation   bool   `json:"allows_cancellation"`
	BatchingSQL          string `json:"batching_sql,omitempty"`
	ProgressTracking     string `json:"progress_tracking,omitempty"`
}

// TableBloatImpact represents table bloat caused by data modifications
type TableBloatImpact struct {
	EstimatedBloatPercent int     `json:"estimated_bloat_percent"`
	DeadTupleCount        int64   `json:"dead_tuple_count"`
	SpaceReclaimableBytes int64   `json:"space_reclaimable_bytes"`
	VacuumRequired        bool    `json:"vacuum_required"`
	VacuumDurationSeconds float64 `json:"vacuum_duration_seconds,omitempty"`
	VacuumRecommendation  string  `json:"vacuum_recommendation,omitempty"`
}
