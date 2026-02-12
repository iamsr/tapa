package integration_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/internal/analyzer/datamigration"
	"github.com/iamsr/tapa/internal/analyzer/diskspace"
	"github.com/iamsr/tapa/internal/analyzer/postgres"
	"github.com/iamsr/tapa/internal/analyzer/rollback"
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

func TestConcurrencyAnalysis_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	migrationSQL := `
	-- High impact: ACCESS EXCLUSIVE lock on large table
	ALTER TABLE users ADD COLUMN last_login TIMESTAMP DEFAULT NOW();

	-- Low impact: Concurrent index creation
	CREATE INDEX CONCURRENTLY idx_users_email ON users(email);

	-- Medium impact: Regular index creation
	CREATE INDEX idx_orders_user_id ON orders(user_id);
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

	// Pre-populate necessary fields for concurrency analysis
	// (normally done by postgres analyzer, but we're testing concurrency analyzer in isolation)
	if len(migration.Operations) >= 1 {
		// ADD COLUMN with DEFAULT NOW() - requires rewrite, ACCESS EXCLUSIVE lock
		migration.Operations[0].RequiresRewrite = true
		migration.Operations[0].LockType = models.LockTypeAccessExclusive
		migration.Operations[0].RowCount = 5000000 // 5M rows for realistic scoring
	}
	if len(migration.Operations) >= 2 {
		// CREATE INDEX CONCURRENTLY - SHARE UPDATE EXCLUSIVE lock
		migration.Operations[1].LockType = models.LockTypeShareUpdateExclusive
		migration.Operations[1].RowCount = 5000000
	}
	if len(migration.Operations) >= 3 {
		// Regular CREATE INDEX - SHARE lock
		migration.Operations[2].LockType = models.LockTypeShare
		migration.Operations[2].RowCount = 10000000 // 10M rows
	}

	// Create concurrency analyzer (nil DB for mock mode)
	analyzer := concurrency.NewAnalyzer("postgresql", nil)

	// Analyze all operations
	ctx := context.Background()
	err = analyzer.AnalyzeMigration(ctx, migration)
	if err != nil {
		t.Fatalf("AnalyzeMigration failed: %v", err)
	}

	// Test high impact operation
	t.Run("HighImpactOperation", func(t *testing.T) {
		op := migration.Operations[0]
		if op.ConcurrencyAnalysis == nil {
			t.Fatal("ConcurrencyAnalysis should be populated")
		}

		if !op.ConcurrencyAnalysis.IsHighImpact() {
			t.Errorf("Expected high impact for ADD COLUMN with DEFAULT, got score %d",
				op.ConcurrencyAnalysis.ImpactScore)
		}

		if op.ConcurrencyAnalysis.ConcurrencySafe {
			t.Error("ADD COLUMN with DEFAULT should not be concurrency safe")
		}

		if len(op.ConcurrencyAnalysis.SaferAlternatives) == 0 {
			t.Error("Expected safer alternatives for high-impact operation")
		}
	})

	// Test concurrent index creation
	t.Run("ConcurrentIndexLowImpact", func(t *testing.T) {
		if len(migration.Operations) < 2 {
			t.Skip("Not enough operations parsed")
		}

		op := migration.Operations[1]
		if op.ConcurrencyAnalysis == nil {
			t.Fatal("ConcurrencyAnalysis should be populated")
		}

		if op.ConcurrencyAnalysis.IsHighImpact() {
			t.Errorf("CONCURRENT index should have low impact, got score %d",
				op.ConcurrencyAnalysis.ImpactScore)
		}
	})

	// Test alternatives generation
	t.Run("AlternativesGenerated", func(t *testing.T) {
		if len(migration.Operations) < 3 {
			t.Skip("Not enough operations parsed")
		}

		op := migration.Operations[2]
		if op.ConcurrencyAnalysis == nil {
			t.Fatal("ConcurrencyAnalysis should be populated")
		}

		// Regular CREATE INDEX should have CONCURRENT alternative
		foundConcurrentAlt := false
		for _, alt := range op.ConcurrencyAnalysis.SaferAlternatives {
			if alt.LockType == models.LockTypeShareUpdateExclusive {
				foundConcurrentAlt = true
				break
			}
		}

		if !foundConcurrentAlt {
			t.Error("Expected CONCURRENT alternative for CREATE INDEX")
		}
	})
}

func TestConcurrencyAnalysis_ComprehensiveMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that comprehensive mode includes concurrency analysis
	migrationSQL := `ALTER TABLE products ADD COLUMN price DECIMAL(10,2) NOT NULL DEFAULT 0.00;`

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

	// Simulate comprehensive mode (all analyzers enabled)
	ctx := context.Background()

	// Mock table stats for analyzers that need it
	stats := &db.TableStats{
		TableName:      "products",
		RowCount:       1000000,
		TableSizeBytes: 10 * 1024 * 1024 * 1024, // 10GB
		IndexSizeBytes: 2 * 1024 * 1024 * 1024,  // 2GB
	}

	diskAnalyzer := diskspace.NewAnalyzer("postgresql", 200)
	rollbackAnalyzer := rollback.NewAnalyzer("postgresql")
	dataMigrationAnalyzer := datamigration.NewAnalyzer("postgresql")
	concurrencyAnalyzer := concurrency.NewAnalyzer("postgresql", nil)

	for _, op := range migration.Operations {
		_ = diskAnalyzer.AnalyzeDiskSpace(ctx, op, stats)
		_ = rollbackAnalyzer.AnalyzeRollback(ctx, op)
		_ = dataMigrationAnalyzer.DetectDataMigration(ctx, op, stats)
		_ = concurrencyAnalyzer.AnalyzeOperation(ctx, op)
	}

	op := migration.Operations[0]

	// Verify all analyses that should be present for ADD COLUMN are present
	if op.DiskSpaceAnalysis == nil {
		t.Error("DiskSpaceAnalysis should be populated in comprehensive mode")
	}

	if op.RollbackAnalysis == nil {
		t.Error("RollbackAnalysis should be populated in comprehensive mode")
	}

	// Note: DataMigrationAnalysis is only populated for UPDATE/INSERT/DELETE operations,
	// not for DDL like ADD COLUMN, so we don't check it here

	if op.ConcurrencyAnalysis == nil {
		t.Error("ConcurrencyAnalysis should be populated in comprehensive mode")
	}
}
