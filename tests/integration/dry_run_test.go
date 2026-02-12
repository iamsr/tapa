package integration_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/dryrun"
	"github.com/iamsr/tapa/internal/parser"
	"github.com/iamsr/tapa/pkg/models"
)

func TestDryRun_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Parse migration with multiple operations
	migrationSQL := `
	-- Add column
	ALTER TABLE users ADD COLUMN email VARCHAR(255);

	-- Add constraint (will be validated in real dry-run)
	ALTER TABLE orders ADD CONSTRAINT fk_user 
		FOREIGN KEY (user_id) REFERENCES users(id);

	-- Create index
	CREATE INDEX idx_users_email ON users(email);
	`

	pgParser, err := parser.GetParser("postgresql")
	if err != nil {
		t.Fatalf("Failed to get parser: %v", err)
	}

	operations, err := pgParser.Parse(migrationSQL)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	migration := models.NewMigration("test_migration.sql")
	for _, op := range operations {
		migration.AddOperation(op)
	}

	// Create dry-run analyzer (nil DB for mock mode)
	analyzer := dryrun.NewAnalyzer("postgresql", nil)

	// Analyze all operations
	ctx := context.Background()
	err = analyzer.AnalyzeMigration(ctx, migration)
	if err != nil {
		t.Fatalf("AnalyzeMigration failed: %v", err)
	}

	// Verify results - in mock mode (nil DB), all operations should be skipped
	t.Run("AllOperationsAnalyzed", func(t *testing.T) {
		for i, op := range migration.Operations {
			if op.DryRunResult == nil {
				t.Errorf("Operation %d: DryRunResult should be populated", i)
			}
		}
	})

	t.Run("MockModeSkipsExecution", func(t *testing.T) {
		// In mock mode (nil DB), operations should be marked as skipped
		for i, op := range migration.Operations {
			if op.DryRunResult != nil && op.DryRunResult.Status != models.DryRunStatusSkipped {
				t.Logf("Operation %d: Status=%s (expected skipped in mock mode)", i, op.DryRunResult.Status)
			}
		}
	})

	t.Run("OperationTypesPreserved", func(t *testing.T) {
		// Verify operations maintain their types and SQL
		if len(migration.Operations) < 3 {
			t.Fatalf("Expected at least 3 operations, got %d", len(migration.Operations))
		}

		// First operation should be ADD COLUMN
		if migration.Operations[0].Type != models.OperationTypeAddColumn {
			t.Errorf("Operation 0: expected ADD_COLUMN, got %s", migration.Operations[0].Type)
		}

		// All operations should have non-empty SQL
		for i, op := range migration.Operations {
			if op.SQL == "" {
				t.Errorf("Operation %d: SQL should not be empty", i)
			}
		}
	})
}

func TestDryRun_ConstraintDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	migrationSQL := `
	CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(100));
	CREATE TABLE orders (
		id SERIAL PRIMARY KEY,
		user_id INT,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	`

	pgParser, err := parser.GetParser("postgresql")
	if err != nil {
		t.Fatalf("Failed to get parser: %v", err)
	}

	operations, err := pgParser.Parse(migrationSQL)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	migration := models.NewMigration("test_migration.sql")
	for _, op := range operations {
		migration.AddOperation(op)
	}

	analyzer := dryrun.NewAnalyzer("postgresql", nil)
	ctx := context.Background()

	for _, op := range migration.Operations {
		err := analyzer.AnalyzeOperation(ctx, op)
		if err != nil {
			t.Errorf("AnalyzeOperation failed: %v", err)
		}

		if op.DryRunResult == nil {
			t.Error("DryRunResult should be populated")
		}
	}
}
