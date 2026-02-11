package rollback

import (
	"context"
	"fmt"
	"strings"

	"github.com/iamsr/tapa/pkg/models"
)

// Analyzer analyzes rollback capabilities for migrations
type Analyzer struct {
	databaseType string
}

// NewAnalyzer creates a new rollback analyzer
func NewAnalyzer(databaseType string) *Analyzer {
	return &Analyzer{
		databaseType: databaseType,
	}
}

// AnalyzeRollback analyzes rollback capability for an operation
func (a *Analyzer) AnalyzeRollback(ctx context.Context, op *models.Operation) error {
	analysis := &models.RollbackAnalysis{}

	switch op.Type {
	case models.OperationTypeCreateIndex:
		a.analyzeCreateIndex(op, analysis)

	case models.OperationTypeDropIndex:
		a.analyzeDropIndex(op, analysis)

	case models.OperationTypeAddColumn:
		a.analyzeAddColumn(op, analysis)

	case models.OperationTypeDropColumn:
		a.analyzeDropColumn(op, analysis)

	case models.OperationTypeAlterColumn:
		a.analyzeAlterColumn(op, analysis)

	case models.OperationTypeCreateTable:
		a.analyzeCreateTable(op, analysis)

	case models.OperationTypeDropTable:
		a.analyzeDropTable(op, analysis)

	default:
		analysis.Category = models.ReversibilityConditional
		analysis.ReversibilityScore = 50
		analysis.Reason = "Unknown operation type, manual review required"
	}

	analysis.IsReversible = analysis.CanRollback()
	op.RollbackAnalysis = analysis
	return nil
}

func (a *Analyzer) analyzeCreateIndex(op *models.Operation, analysis *models.RollbackAnalysis) {
	analysis.Category = models.ReversibilitySafe
	analysis.ReversibilityScore = 100
	analysis.Reason = "Index creation is fully reversible with DROP INDEX"

	// Generate auto-rollback SQL
	analysis.AutoRollbackSQL = fmt.Sprintf("DROP INDEX %s;", op.IndexName)
}

func (a *Analyzer) analyzeDropIndex(op *models.Operation, analysis *models.RollbackAnalysis) {
	analysis.Category = models.ReversibilityConditional
	analysis.ReversibilityScore = 75
	analysis.Reason = "Index can be recreated if index definition is known"

	analysis.RecoveryStrategy = &models.RecoveryStrategy{
		Method: "recreate_index",
		Steps: []string{
			"Obtain original CREATE INDEX statement from schema backup",
			"Execute CREATE INDEX (or CREATE INDEX CONCURRENTLY)",
			"Wait for index build to complete",
		},
		Prerequisites: []string{
			"Original index definition must be documented",
		},
	}
}

func (a *Analyzer) analyzeAddColumn(op *models.Operation, analysis *models.RollbackAnalysis) {
	sqlLower := strings.ToLower(op.SQL)

	if strings.Contains(sqlLower, "not null") && !strings.Contains(sqlLower, "default") {
		// NOT NULL without DEFAULT - risky to roll back if data exists
		analysis.Category = models.ReversibilityConditional
		analysis.ReversibilityScore = 60
		analysis.Reason = "Column has NOT NULL constraint, rollback safe only if no data inserted"
	} else {
		analysis.Category = models.ReversibilitySafe
		analysis.ReversibilityScore = 95
		analysis.Reason = "Column addition is reversible with DROP COLUMN"
	}

	analysis.AutoRollbackSQL = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", op.TableName, op.ColumnName)
}

func (a *Analyzer) analyzeDropColumn(op *models.Operation, analysis *models.RollbackAnalysis) {
	analysis.Category = models.ReversibilityIrreversible
	analysis.ReversibilityScore = 0
	analysis.Reason = "Column data is permanently deleted"

	analysis.RecoveryStrategy = &models.RecoveryStrategy{
		Method:       "backup_restore",
		BackupMethod: "pg_dump or filesystem snapshot",
		EstimatedRTO: "Depends on database size (minutes to hours)",
		Steps: []string{
			"Restore database from backup taken before migration",
			"Verify data integrity",
			"Consider alternative: rename column instead of dropping",
		},
		AlternativeApproach: fmt.Sprintf(
			"Instead of DROP COLUMN, consider: ALTER TABLE %s RENAME COLUMN %s TO %s_deprecated;",
			op.TableName, op.ColumnName, op.ColumnName,
		),
	}

	analysis.DataAffected = &models.DataImpact{
		AffectedColumns: []string{op.ColumnName},
		DataLossType:    "complete_loss",
		IsRecoverable:   false,
	}
}

func (a *Analyzer) analyzeAlterColumn(op *models.Operation, analysis *models.RollbackAnalysis) {
	sqlLower := strings.ToLower(op.SQL)

	// Detect type changes that cause data loss
	if a.detectDataLossTypeChange(sqlLower) {
		analysis.Category = models.ReversibilityDataLoss
		analysis.ReversibilityScore = 25
		analysis.Reason = "Type conversion may cause precision or data loss"

		analysis.RecoveryStrategy = &models.RecoveryStrategy{
			Method:              "backup_restore",
			BackupMethod:        "pg_dump before migration",
			EstimatedRTO:        "Depends on database size",
			AlternativeApproach: "Use multi-column approach: add new column, migrate data, rename columns",
		}

		analysis.DataAffected = &models.DataImpact{
			AffectedColumns: []string{op.ColumnName},
			DataLossType:    "precision_loss",
			IsRecoverable:   false,
		}
	} else {
		// Safe type changes (e.g., VARCHAR to TEXT, smaller to larger types)
		analysis.Category = models.ReversibilityConditional
		analysis.ReversibilityScore = 70
		analysis.Reason = "Type change may be reversible if no data overflow"
	}
}

func (a *Analyzer) analyzeCreateTable(op *models.Operation, analysis *models.RollbackAnalysis) {
	analysis.Category = models.ReversibilitySafe
	analysis.ReversibilityScore = 100
	analysis.Reason = "Table creation is fully reversible with DROP TABLE"

	analysis.AutoRollbackSQL = fmt.Sprintf("DROP TABLE %s;", op.TableName)
}

func (a *Analyzer) analyzeDropTable(op *models.Operation, analysis *models.RollbackAnalysis) {
	analysis.Category = models.ReversibilityIrreversible
	analysis.ReversibilityScore = 0
	analysis.Reason = "All table data is permanently deleted"

	analysis.RecoveryStrategy = &models.RecoveryStrategy{
		Method:       "backup_restore",
		BackupMethod: "pg_dump or filesystem snapshot",
		EstimatedRTO: "Depends on database size (minutes to hours)",
		Steps: []string{
			"Restore entire database from backup",
			"Or restore specific table using pg_restore with --table option",
			"Verify foreign key relationships",
		},
	}

	analysis.DataAffected = &models.DataImpact{
		DataLossType:  "complete_loss",
		IsRecoverable: false,
	}
}

// detectDataLossTypeChange detects type conversions that cause data loss
func (a *Analyzer) detectDataLossTypeChange(sqlLower string) bool {
	dataLossPatterns := []string{
		"numeric.*integer", // NUMERIC to INTEGER
		"decimal.*int",     // DECIMAL to INT
		"text.*varchar",    // TEXT to VARCHAR (potential truncation)
		"timestamp.*date",  // TIMESTAMP to DATE (time loss)
		"double.*float",    // DOUBLE to FLOAT (precision loss)
		"bigint.*integer",  // BIGINT to INTEGER (overflow risk)
	}

	for _, pattern := range dataLossPatterns {
		if strings.Contains(sqlLower, pattern) {
			return true
		}
	}

	// Check for potentially lossy type conversions (when we don't have source type)
	// ALTER COLUMN TYPE INTEGER on a column named "total" suggests numeric→integer conversion
	if strings.Contains(sqlLower, "type integer") || strings.Contains(sqlLower, "type int") {
		// Heuristic: if it's a numeric-sounding column, assume data loss risk
		if strings.Contains(sqlLower, "total") ||
			strings.Contains(sqlLower, "price") ||
			strings.Contains(sqlLower, "amount") ||
			strings.Contains(sqlLower, "balance") {
			return true
		}
	}

	return false
}
