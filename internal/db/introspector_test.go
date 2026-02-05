package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockIntrospector is a test implementation of the Introspector interface
type mockIntrospector struct {
	connected bool
}

func (m *mockIntrospector) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *mockIntrospector) Close() error {
	m.connected = false
	return nil
}

func (m *mockIntrospector) GetTableStats(ctx context.Context, tableName string) (*TableStats, error) {
	return &TableStats{
		TableName:      tableName,
		RowCount:       100,
		TableSizeBytes: 1024,
		IndexSizeBytes: 512,
		Indexes:        []IndexInfo{},
	}, nil
}

func (m *mockIntrospector) TableExists(ctx context.Context, tableName string) (bool, error) {
	return tableName == "users", nil
}

func TestIntrospectorInterface(t *testing.T) {
	var _ Introspector = (*mockIntrospector)(nil)

	mock := &mockIntrospector{}
	ctx := context.Background()

	// Test Connect
	err := mock.Connect(ctx)
	assert.NoError(t, err)
	assert.True(t, mock.connected)

	// Test TableExists
	exists, err := mock.TableExists(ctx, "users")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = mock.TableExists(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.False(t, exists)

	// Test GetTableStats
	stats, err := mock.GetTableStats(ctx, "users")
	assert.NoError(t, err)
	assert.Equal(t, "users", stats.TableName)
	assert.Equal(t, int64(100), stats.RowCount)
	assert.Greater(t, stats.TableSizeBytes, int64(0))

	// Test Close
	err = mock.Close()
	assert.NoError(t, err)
	assert.False(t, mock.connected)
}
