package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/dma/internal/db"
	"github.com/yourusername/dma/pkg/models"
)

// mockIntrospector for testing
type mockIntrospector struct {
	stats *db.TableStats
	err   error
}

func (m *mockIntrospector) Connect(ctx context.Context) error { return nil }
func (m *mockIntrospector) Close() error                      { return nil }
func (m *mockIntrospector) TableExists(ctx context.Context, name string) (bool, error) {
	return true, nil
}
func (m *mockIntrospector) GetTableStats(ctx context.Context, name string) (*db.TableStats, error) {
	return m.stats, m.err
}

func TestAnalyzer_AddColumnWithoutDefault(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "users",
			RowCount:       1000,
			TableSizeBytes: 1024 * 1024 * 100, // 100 MB
		},
	}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// ADD COLUMN without DEFAULT should be fast with ACCESS EXCLUSIVE lock
	assert.Equal(t, models.LockTypeAccessExclusive, op.LockType)
	assert.False(t, op.RequiresRewrite, "Should not require table rewrite")
	assert.Less(t, op.RiskScore, 50, "Small table should have low-medium risk")
	assert.True(t, op.BackwardCompatible, "Adding nullable column is backward compatible")
}

func TestAnalyzer_AddColumnWithConstantDefault(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "users",
			RowCount:       10000000,                // 10M rows
			TableSizeBytes: 50 * 1024 * 1024 * 1024, // 50 GB
		},
	}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'active'",
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// Large table with DEFAULT should have high risk
	assert.Equal(t, models.LockTypeAccessExclusive, op.LockType)
	assert.True(t, op.RequiresRewrite, "DEFAULT value requires rewrite")
	assert.Greater(t, op.RiskScore, 50, "Large table rewrite is high risk")
	assert.Greater(t, op.EstimatedTimeSeconds, 100.0, "50GB should take significant time")
	// Check that recommendations contain the key advice
	found := false
	for _, rec := range op.Recommendations {
		if strings.Contains(rec, "Add column without DEFAULT first") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should recommend avoiding rewrite by adding column without DEFAULT first")
}

func TestAnalyzer_AlterColumnType(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "orders",
			RowCount:       5000000,
			TableSizeBytes: 20 * 1024 * 1024 * 1024, // 20 GB
		},
	}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "ALTER TABLE orders ALTER COLUMN total TYPE NUMERIC(12,2)",
		Type:      models.OperationTypeAlterColumn,
		TableName: "orders",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// ALTER COLUMN TYPE requires full table rewrite
	assert.Equal(t, models.LockTypeAccessExclusive, op.LockType)
	assert.True(t, op.RequiresRewrite, "Type change requires rewrite")
	assert.Greater(t, op.RiskScore, 50, "Large table rewrite is high risk")
	assert.False(t, op.BackwardCompatible, "Type change breaks compatibility")
}

func TestAnalyzer_CreateIndexConcurrently(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "users",
			RowCount:       1000000,
			TableSizeBytes: 10 * 1024 * 1024 * 1024, // 10 GB
		},
	}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
		Type:      models.OperationTypeCreateIndex,
		TableName: "users",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// CONCURRENTLY should have minimal locks and low risk
	assert.Equal(t, models.LockTypeShareUpdateExclusive, op.LockType)
	assert.Less(t, op.RiskScore, 50, "CONCURRENTLY index is low-medium risk")
	assert.True(t, op.BackwardCompatible, "Adding index is backward compatible")
}

func TestAnalyzer_CreateIndexWithoutConcurrently(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "users",
			RowCount:       10000000,
			TableSizeBytes: 50 * 1024 * 1024 * 1024, // 50 GB
		},
	}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "CREATE INDEX idx_users_email ON users(email)",
		Type:      models.OperationTypeCreateIndex,
		TableName: "users",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// Without CONCURRENTLY on large table is high risk
	assert.Equal(t, models.LockTypeShare, op.LockType)
	assert.Greater(t, op.RiskScore, 50, "SHARE lock on large table is high risk")

	// Check that recommendations contain the key advice
	found := false
	for _, rec := range op.Recommendations {
		if strings.Contains(rec, "Use CREATE INDEX CONCURRENTLY") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should recommend using CONCURRENTLY")
}

func TestAnalyzer_DropColumn(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "users",
			RowCount:       1000,
			TableSizeBytes: 1024 * 1024 * 10, // 10 MB
		},
	}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "ALTER TABLE users DROP COLUMN deprecated_field",
		Type:      models.OperationTypeDropColumn,
		TableName: "users",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// DROP COLUMN is instant but exclusive lock
	assert.Equal(t, models.LockTypeAccessExclusive, op.LockType)
	assert.False(t, op.RequiresRewrite, "DROP COLUMN doesn't rewrite in PG 11+")
	assert.False(t, op.BackwardCompatible, "Dropping column breaks compatibility")
}

func TestAnalyzer_DropTable(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "temp_table",
			RowCount:       100,
			TableSizeBytes: 1024 * 1024, // 1 MB
		},
	}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "DROP TABLE temp_table",
		Type:      models.OperationTypeDropTable,
		TableName: "temp_table",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// DROP TABLE is instant with exclusive lock
	assert.Equal(t, models.LockTypeAccessExclusive, op.LockType)
	assert.False(t, op.RequiresRewrite)
	assert.False(t, op.BackwardCompatible, "Dropping table breaks compatibility")
	assert.Less(t, op.EstimatedTimeSeconds, 1.0, "Should be very fast")
}

func TestAnalyzer_CreateTable(t *testing.T) {
	introspector := &mockIntrospector{}

	analyzer := NewAnalyzer(introspector, 200, 2.0)

	op := &models.Operation{
		SQL:       "CREATE TABLE new_table (id SERIAL PRIMARY KEY, name VARCHAR(255))",
		Type:      models.OperationTypeCreateTable,
		TableName: "new_table",
	}

	err := analyzer.Analyze(context.Background(), op)
	require.NoError(t, err)

	// CREATE TABLE doesn't lock existing tables
	assert.Equal(t, models.LockTypeNone, op.LockType)
	assert.False(t, op.RequiresRewrite)
	assert.True(t, op.BackwardCompatible, "Creating new table is backward compatible")
	assert.Less(t, op.RiskScore, 26, "Should be low risk")
}

func TestAnalyzer_RiskScoreCalculation(t *testing.T) {
	tests := []struct {
		name              string
		sql               string
		opType            models.OperationType
		tableSizeBytes    int64
		expectedRiskLevel models.RiskLevel
	}{
		{
			name:              "small table, fast operation",
			sql:               "ALTER TABLE test_table ADD COLUMN col VARCHAR(255)",
			opType:            models.OperationTypeAddColumn,
			tableSizeBytes:    100 * 1024 * 1024,      // 100 MB
			expectedRiskLevel: models.RiskLevelMedium, // ACCESS EXCLUSIVE on small table with rewrite
		},
		{
			name:              "medium table, rewrite required",
			sql:               "ALTER TABLE test_table ALTER COLUMN col TYPE VARCHAR(255)",
			opType:            models.OperationTypeAlterColumn,
			tableSizeBytes:    5 * 1024 * 1024 * 1024, // 5 GB
			expectedRiskLevel: models.RiskLevelHigh,   // 40 + 15 + 0 = 55
		},
		{
			name:              "large table, rewrite required",
			sql:               "ALTER TABLE test_table ALTER COLUMN col TYPE VARCHAR(255)",
			opType:            models.OperationTypeAlterColumn,
			tableSizeBytes:    20 * 1024 * 1024 * 1024, // 20 GB
			expectedRiskLevel: models.RiskLevelHigh,
		},
		{
			name:              "very large table, rewrite required",
			sql:               "ALTER TABLE test_table ALTER COLUMN col TYPE VARCHAR(255)",
			opType:            models.OperationTypeAlterColumn,
			tableSizeBytes:    100 * 1024 * 1024 * 1024, // 100 GB
			expectedRiskLevel: models.RiskLevelCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			introspector := &mockIntrospector{
				stats: &db.TableStats{
					TableName:      "test_table",
					RowCount:       1000000,
					TableSizeBytes: tt.tableSizeBytes,
				},
			}

			analyzer := NewAnalyzer(introspector, 200, 2.0)

			op := &models.Operation{
				SQL:       tt.sql,
				Type:      tt.opType,
				TableName: "test_table",
			}

			err := analyzer.Analyze(context.Background(), op)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedRiskLevel, op.RiskLevel(), "Risk level should match expected")
		})
	}
}
