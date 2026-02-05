package analyzer

import (
	"context"
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

func TestGetAnalyzer(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "users",
			RowCount:       1000,
			TableSizeBytes: 1024 * 1024 * 100, // 100 MB
		},
	}

	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{"postgresql", "postgresql", false},
		{"mysql not implemented", "mysql", true},
		{"unsupported", "oracle", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer, err := GetAnalyzer(tt.dbType, introspector, 200, 2.0)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, analyzer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, analyzer)
			}
		})
	}
}

func TestAnalyzer_Interface(t *testing.T) {
	introspector := &mockIntrospector{
		stats: &db.TableStats{
			TableName:      "users",
			RowCount:       1000,
			TableSizeBytes: 1024 * 1024 * 100, // 100 MB
		},
	}

	analyzer, err := GetAnalyzer("postgresql", introspector, 200, 2.0)
	require.NoError(t, err)
	require.NotNil(t, analyzer)

	// Test that it implements the Analyzer interface
	op := &models.Operation{
		SQL:       "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
	}

	err = analyzer.Analyze(context.Background(), op)
	assert.NoError(t, err)

	// After analysis, operation should have risk data populated
	assert.NotEqual(t, models.LockTypeNone, op.LockType, "Lock type should be set")
	assert.Greater(t, op.RiskScore, 0, "Risk score should be calculated")
}
