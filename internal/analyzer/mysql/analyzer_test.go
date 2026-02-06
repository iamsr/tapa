package mysql

import (
	"context"
	"testing"

	"github.com/yourusername/dma/pkg/models"
)

func TestAnalyzer_AddColumn_LockDetection(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	ctx := context.Background()

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN status VARCHAR(50)",
	}

	err := analyzer.Analyze(ctx, op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// ADD COLUMN without DEFAULT uses INPLACE algorithm, no lock
	if op.LockType != models.LockTypeNone {
		t.Errorf("Expected LockType NONE, got %s", op.LockType)
	}

	if op.RequiresRewrite {
		t.Errorf("Expected RequiresRewrite false, got true")
	}
}

func TestAnalyzer_AddColumn_WithDefault_LockDetection(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	ctx := context.Background()

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN status VARCHAR(50) DEFAULT 'active'",
	}

	err := analyzer.Analyze(ctx, op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// ADD COLUMN with DEFAULT requires COPY algorithm (MySQL 5.7) or INSTANT (8.0+)
	// For safety, we assume it requires exclusive lock and rewrite
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected LockType EXCLUSIVE, got %s", op.LockType)
	}

	if !op.RequiresRewrite {
		t.Errorf("Expected RequiresRewrite true, got false")
	}
}
