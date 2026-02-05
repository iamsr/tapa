package models

import "time"

// OperationType represents the type of DDL operation
type OperationType string

const (
	OperationTypeAlterTable  OperationType = "ALTER_TABLE"
	OperationTypeCreateTable OperationType = "CREATE_TABLE"
	OperationTypeDropTable   OperationType = "DROP_TABLE"
	OperationTypeCreateIndex OperationType = "CREATE_INDEX"
	OperationTypeDropIndex   OperationType = "DROP_INDEX"
	OperationTypeAddColumn   OperationType = "ADD_COLUMN"
	OperationTypeDropColumn  OperationType = "DROP_COLUMN"
	OperationTypeAlterColumn OperationType = "ALTER_COLUMN"
	OperationTypeUnknown     OperationType = "UNKNOWN"
)

// LockType represents database lock types
type LockType string

const (
	LockTypeAccessExclusive      LockType = "ACCESS_EXCLUSIVE"
	LockTypeShareUpdateExclusive LockType = "SHARE_UPDATE_EXCLUSIVE"
	LockTypeShare                LockType = "SHARE"
	LockTypeExclusive            LockType = "EXCLUSIVE"
	LockTypeRowExclusive         LockType = "ROW_EXCLUSIVE"
	LockTypeNone                 LockType = "NONE"
)

// RiskLevel represents risk assessment categories
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"
	RiskLevelMedium   RiskLevel = "MEDIUM"
	RiskLevelHigh     RiskLevel = "HIGH"
	RiskLevelCritical RiskLevel = "CRITICAL"
)

// Operation represents a single DDL operation within a migration
type Operation struct {
	SQL                  string
	Type                 OperationType
	TableName            string
	LockType             LockType
	LockDurationMS       int64
	RequiresRewrite      bool
	EstimatedTimeSeconds float64
	RiskScore            int
	BackwardCompatible   bool
	Recommendations      []string
}

// IsHighRisk returns true if risk score >= 51
func (o *Operation) IsHighRisk() bool {
	return o.RiskScore >= 51
}

// RiskLevel returns the risk category based on score
func (o *Operation) RiskLevel() RiskLevel {
	switch {
	case o.RiskScore >= 76:
		return RiskLevelCritical
	case o.RiskScore >= 51:
		return RiskLevelHigh
	case o.RiskScore >= 26:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

// EstimatedDuration returns the estimated time as a duration
func (o *Operation) EstimatedDuration() time.Duration {
	return time.Duration(o.EstimatedTimeSeconds * float64(time.Second))
}
