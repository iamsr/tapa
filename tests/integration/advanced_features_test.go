package integration_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/postgres"
	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/internal/parser"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAdvancedFeatures_EndToEnd(t *testing.T) {
	// Skip if short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Parse migration with multiple operations
	migrationSQL := `
	-- Add column (requires disk space)
	ALTER TABLE users ADD COLUMN full_name TEXT DEFAULT 'unknown';

	-- Drop column (irreversible)
	ALTER TABLE orders DROP COLUMN old_status;

	-- Create index (safe and reversible)
	CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
	`

	pgParser, err := parser.GetParser("postgresql")
	if err != nil {
		t.Fatalf("Failed to get parser: %v", err)
	}

	operations, err := pgParser.Parse(migrationSQL)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(operations) != 3 {
		for i, op := range operations {
			t.Logf("Operation %d: Type=%s, SQL=%s", i, op.Type, op.SQL)
		}
		t.Fatalf("Expected 3 operations, got %d", len(operations))
	}

	migration := models.NewMigration("test_migration.sql")
	for _, op := range operations {
		migration.AddOperation(op)
	}

	// Create analyzer in comprehensive mode
	analyzer := postgres.NewAnalyzer(nil, 200, 2.0, true)

	// Mock table stats
	statsMap := map[string]*db.TableStats{
		"users": {
			TableName:      "users",
			RowCount:       5000000,
			TableSizeBytes: 80 * GB,
			IndexSizeBytes: 15 * GB,
		},
		"orders": {
			TableName:      "orders",
			RowCount:       10000000,
			TableSizeBytes: 120 * GB,
			IndexSizeBytes: 25 * GB,
		},
	}

	// Analyze all operations
	ctx := context.Background()
	for _, op := range migration.Operations {
		// Inject mock stats
		if stats, ok := statsMap[op.TableName]; ok {
			// Would normally be done by analyzer with introspector
			op.RowCount = stats.RowCount
			op.TableSizeBytes = stats.TableSizeBytes
		}

		err := analyzer.Analyze(ctx, op)
		if err != nil {
			t.Fatalf("Analyze failed for operation %s: %v", op.Type, err)
		}
	}

	// Verify advanced features for each operation
	t.Run("AddColumn_DiskSpace", func(t *testing.T) {
		op := migration.Operations[0] // ADD COLUMN
		if op.DiskSpaceAnalysis == nil {
			t.Error("DiskSpaceAnalysis should be populated")
		}
		if op.RollbackAnalysis == nil {
			t.Error("RollbackAnalysis should be populated for ADD COLUMN")
		}
	})

	t.Run("DropColumn_Rollback", func(t *testing.T) {
		op := migration.Operations[1] // DROP COLUMN
		if op.RollbackAnalysis == nil {
			t.Error("RollbackAnalysis should be populated")
		}
		if op.RollbackAnalysis.Category != models.ReversibilityIrreversible {
			t.Errorf("DROP COLUMN should be irreversible, got %s", op.RollbackAnalysis.Category)
		}
	})

	t.Run("CreateIndex_AllFeatures", func(t *testing.T) {
		op := migration.Operations[2] // CREATE INDEX
		if op.DiskSpaceAnalysis == nil {
			t.Error("DiskSpaceAnalysis should be populated")
		}
		if op.RollbackAnalysis == nil {
			t.Error("RollbackAnalysis should be populated")
		}
		if op.RollbackAnalysis.Category != models.ReversibilitySafe {
			t.Errorf("CREATE INDEX should be safe, got %s", op.RollbackAnalysis.Category)
		}
	})
}

const GB = 1024 * 1024 * 1024
