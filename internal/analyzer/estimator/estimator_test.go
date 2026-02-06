package estimator

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/iamsr/dma/internal/db"
	"github.com/iamsr/dma/pkg/models"
)

func TestGetTimeEstimator(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{"postgresql", "postgresql", false},
		{"mysql", "mysql", false},
		{"unsupported", "oracle", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator, err := GetTimeEstimator(tt.dbType, nil, 200, 2.0)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, estimator)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, estimator)
			}
		})
	}
}

func TestPostgresTimeEstimator_EstimateTime_NoTableName(t *testing.T) {
	estimator, _ := GetTimeEstimator("postgresql", nil, 200, 2.0)

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "", // no table name
	}

	breakdown, err := estimator.EstimateTime(context.Background(), op)
	require.NoError(t, err)
	require.NotNil(t, breakdown)

	// Should return minimal estimate
	assert.Equal(t, 0.1, breakdown.MetadataUpdateSeconds)
	assert.Equal(t, 0.1, breakdown.TotalSeconds)
}

func TestPostgresTimeEstimator_EstimateTime_NoIntrospector(t *testing.T) {
	estimator, _ := GetTimeEstimator("postgresql", nil, 200, 2.0)

	op := &models.Operation{
		Type:            models.OperationTypeAlterColumn,
		TableName:       "users",
		RequiresRewrite: true,
	}

	breakdown, err := estimator.EstimateTime(context.Background(), op)
	require.NoError(t, err)
	require.NotNil(t, breakdown)

	// Should return minimal estimate when no introspector
	assert.Equal(t, 0.1, breakdown.MetadataUpdateSeconds)
	assert.Equal(t, 0.1, breakdown.TotalSeconds)
}

func TestPostgresTimeEstimator_EstimateTime_WithRewrite(t *testing.T) {
	mockIntr := &mockIntrospector{
		tableStats: &db.TableStats{
			TableName:      "users",
			RowCount:       100000,
			TableSizeBytes: 100 * 1024 * 1024, // 100 MB
			Indexes: []db.IndexInfo{
				{IndexName: "idx_users_email", IndexType: "btree"},
				{IndexName: "idx_users_name", IndexType: "btree"},
			},
		},
	}

	estimator, _ := GetTimeEstimator("postgresql", mockIntr, 200, 2.0)

	op := &models.Operation{
		Type:            models.OperationTypeAlterColumn,
		TableName:       "users",
		RequiresRewrite: true,
	}

	breakdown, err := estimator.EstimateTime(context.Background(), op)
	require.NoError(t, err)
	require.NotNil(t, breakdown)

	// Verify calculations
	// Table rewrite: (100 MB / 200 MB/s) * 2.0 = 1.0s
	assert.Equal(t, 1.0, breakdown.TableRewriteSeconds)

	// Index rebuild: (100 MB / 200 MB/s) * 2 indexes * 0.5 = 0.5s
	assert.Equal(t, 0.5, breakdown.IndexBuildSeconds)

	// Metadata: 0.1s
	assert.Equal(t, 0.1, breakdown.MetadataUpdateSeconds)

	// Total: 1.0 + 0.5 + 0.1 = 1.6s
	assert.Equal(t, 1.6, breakdown.TotalSeconds)
}

func TestPostgresTimeEstimator_EstimateTime_NoRewrite(t *testing.T) {
	mockIntr := &mockIntrospector{
		tableStats: &db.TableStats{
			TableName:      "users",
			RowCount:       100000,
			TableSizeBytes: 100 * 1024 * 1024,
			Indexes:        []db.IndexInfo{},
		},
	}

	estimator, _ := GetTimeEstimator("postgresql", mockIntr, 200, 2.0)

	op := &models.Operation{
		Type:            models.OperationTypeAddColumn,
		TableName:       "users",
		RequiresRewrite: false, // no rewrite needed
	}

	breakdown, err := estimator.EstimateTime(context.Background(), op)
	require.NoError(t, err)
	require.NotNil(t, breakdown)

	// No rewrite means only metadata update
	assert.Equal(t, 0.0, breakdown.TableRewriteSeconds)
	assert.Equal(t, 0.0, breakdown.IndexBuildSeconds)
	assert.Equal(t, 0.1, breakdown.MetadataUpdateSeconds)
	assert.Equal(t, 0.1, breakdown.TotalSeconds)
}

func TestPostgresTimeEstimator_EstimateTime_GetTableStatsError(t *testing.T) {
	mockIntr := &mockIntrospector{
		tableStatsError: fmt.Errorf("connection timeout"),
	}

	estimator, _ := GetTimeEstimator("postgresql", mockIntr, 200, 2.0)

	op := &models.Operation{
		Type:            models.OperationTypeAlterColumn,
		TableName:       "users",
		RequiresRewrite: true,
	}

	breakdown, err := estimator.EstimateTime(context.Background(), op)
	require.NoError(t, err) // Should not error, just return minimal estimate
	require.NotNil(t, breakdown)

	// Should fallback to minimal estimate
	assert.Equal(t, 0.1, breakdown.MetadataUpdateSeconds)
	assert.Equal(t, 0.1, breakdown.TotalSeconds)
}

func TestPostgresTimeEstimator_InputValidation(t *testing.T) {
	// Test zero throughput gets default
	estimator, _ := GetTimeEstimator("postgresql", nil, 0, 2.0)
	assert.NotNil(t, estimator)

	// Test negative throughput gets default
	estimator, _ = GetTimeEstimator("postgresql", nil, -100, 2.0)
	assert.NotNil(t, estimator)

	// Test zero rewrite factor gets default
	estimator, _ = GetTimeEstimator("postgresql", nil, 200, 0)
	assert.NotNil(t, estimator)

	// Test negative rewrite factor gets default
	estimator, _ = GetTimeEstimator("postgresql", nil, 200, -1.0)
	assert.NotNil(t, estimator)
}

// Mock introspector for testing
type mockIntrospector struct {
	tableStats      *db.TableStats
	tableStatsError error
}

func (m *mockIntrospector) Connect(ctx context.Context) error { return nil }
func (m *mockIntrospector) Close() error                      { return nil }
func (m *mockIntrospector) TableExists(ctx context.Context, name string) (bool, error) {
	return true, nil
}
func (m *mockIntrospector) GetTableStats(ctx context.Context, name string) (*db.TableStats, error) {
	if m.tableStatsError != nil {
		return nil, m.tableStatsError
	}
	if m.tableStats != nil {
		return m.tableStats, nil
	}
	return &db.TableStats{}, nil
}
