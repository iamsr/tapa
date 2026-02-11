package models

// ReversibilityCategory represents how easily a migration can be rolled back
type ReversibilityCategory string

const (
	ReversibilitySafe         ReversibilityCategory = "SAFE"
	ReversibilityConditional  ReversibilityCategory = "CONDITIONAL"
	ReversibilityDataLoss     ReversibilityCategory = "DATA LOSS"
	ReversibilityIrreversible ReversibilityCategory = "IRREVERSIBLE"
)

// RollbackAnalysis represents rollback information for a migration
type RollbackAnalysis struct {
	Category           ReversibilityCategory `json:"category"`
	ReversibilityScore int                   `json:"reversibility_score"` // 0-100, higher is more reversible
	IsReversible       bool                  `json:"is_reversible"`
	Reason             string                `json:"reason"`
	AutoRollbackSQL    string                `json:"auto_rollback_sql,omitempty"`
	RecoveryStrategy   *RecoveryStrategy     `json:"recovery_strategy,omitempty"`
	DataAffected       *DataImpact           `json:"data_affected,omitempty"`
}

// RecoveryStrategy provides guidance for irreversible migrations
type RecoveryStrategy struct {
	Method              string   `json:"method"` // "backup_restore", "point_in_time", "multi_step"
	BackupMethod        string   `json:"backup_method,omitempty"`
	EstimatedRTO        string   `json:"estimated_rto,omitempty"` // Recovery Time Objective
	Steps               []string `json:"steps,omitempty"`
	Prerequisites       []string `json:"prerequisites,omitempty"`
	AlternativeApproach string   `json:"alternative_approach,omitempty"`
}

// DataImpact describes data affected by irreversible operations
type DataImpact struct {
	EstimatedRows   int64    `json:"estimated_rows"`
	AffectedColumns []string `json:"affected_columns,omitempty"`
	DataLossType    string   `json:"data_loss_type"` // "precision_loss", "complete_loss", "transformation"
	IsRecoverable   bool     `json:"is_recoverable"`
}

// CanRollback returns true if the operation can be rolled back based on category
func (r *RollbackAnalysis) CanRollback() bool {
	return r.Category == ReversibilitySafe || r.Category == ReversibilityConditional
}
