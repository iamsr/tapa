package rollback_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/rollback"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAnalyzer_AnalyzeRollback_CreateIndex(t *testing.T) {
	analyzer := rollback.NewAnalyzer("postgresql")

	op := &models.Operation{
		Type:      models.OperationTypeCreateIndex,
		TableName: "users",
		IndexName: "idx_users_email",
		SQL:       "CREATE INDEX idx_users_email ON users(email)",
	}

	err := analyzer.AnalyzeRollback(context.Background(), op)
	if err != nil {
		t.Fatalf("AnalyzeRollback failed: %v", err)
	}

	if op.RollbackAnalysis == nil {
		t.Fatal("RollbackAnalysis is nil")
	}

	if op.RollbackAnalysis.Category != models.ReversibilitySafe {
		t.Errorf("Category = %v, want SAFE", op.RollbackAnalysis.Category)
	}

	if op.RollbackAnalysis.AutoRollbackSQL == "" {
		t.Error("AutoRollbackSQL should be generated")
	}

	expectedSQL := "DROP INDEX idx_users_email;"
	if op.RollbackAnalysis.AutoRollbackSQL != expectedSQL {
		t.Errorf("AutoRollbackSQL = %v, want %v", op.RollbackAnalysis.AutoRollbackSQL, expectedSQL)
	}
}

func TestAnalyzer_AnalyzeRollback_DropColumn(t *testing.T) {
	analyzer := rollback.NewAnalyzer("postgresql")

	op := &models.Operation{
		Type:       models.OperationTypeDropColumn,
		TableName:  "users",
		ColumnName: "email",
		SQL:        "ALTER TABLE users DROP COLUMN email",
	}

	err := analyzer.AnalyzeRollback(context.Background(), op)
	if err != nil {
		t.Fatalf("AnalyzeRollback failed: %v", err)
	}

	if op.RollbackAnalysis.Category != models.ReversibilityIrreversible {
		t.Errorf("Category = %v, want IRREVERSIBLE", op.RollbackAnalysis.Category)
	}

	if op.RollbackAnalysis.RecoveryStrategy == nil {
		t.Error("RecoveryStrategy should be provided")
	}
}

func TestAnalyzer_AnalyzeRollback_AlterColumnType(t *testing.T) {
	analyzer := rollback.NewAnalyzer("postgresql")

	op := &models.Operation{
		Type:       models.OperationTypeAlterColumn,
		TableName:  "orders",
		ColumnName: "total",
		SQL:        "ALTER TABLE orders ALTER COLUMN total TYPE INTEGER",
	}

	err := analyzer.AnalyzeRollback(context.Background(), op)
	if err != nil {
		t.Fatalf("AnalyzeRollback failed: %v", err)
	}

	// Type change from NUMERIC to INTEGER = data loss
	if op.RollbackAnalysis.Category != models.ReversibilityDataLoss {
		t.Errorf("Category = %v, want DATA LOSS", op.RollbackAnalysis.Category)
	}
}
