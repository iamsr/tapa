//go:build integration
// +build integration

package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDSN(t *testing.T) string {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}
	return dsn
}

func TestIntrospector_Connect(t *testing.T) {
	dsn := getTestDSN(t)
	i := NewIntrospector(dsn)

	err := i.Connect(context.Background())
	require.NoError(t, err)
	defer i.Close()
}

func TestIntrospector_TableExists(t *testing.T) {
	dsn := getTestDSN(t)
	i := NewIntrospector(dsn)

	ctx := context.Background()
	require.NoError(t, i.Connect(ctx))
	defer i.Close()

	// Create test table
	_, err := i.conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS test_users (id SERIAL PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	defer i.conn.Exec(ctx, "DROP TABLE IF EXISTS test_users")

	exists, err := i.TableExists(ctx, "test_users")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = i.TableExists(ctx, "nonexistent_table")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestIntrospector_GetTableStats(t *testing.T) {
	dsn := getTestDSN(t)
	i := NewIntrospector(dsn)

	ctx := context.Background()
	require.NoError(t, i.Connect(ctx))
	defer i.Close()

	// Create and populate test table
	_, err := i.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_stats (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255),
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(t, err)
	defer i.conn.Exec(ctx, "DROP TABLE IF EXISTS test_stats")

	// Insert test data
	_, err = i.conn.Exec(ctx, "INSERT INTO test_stats (email) SELECT 'user' || generate_series(1, 100) || '@example.com'")
	require.NoError(t, err)

	// Create index
	_, err = i.conn.Exec(ctx, "CREATE INDEX idx_stats_email ON test_stats(email)")
	require.NoError(t, err)

	stats, err := i.GetTableStats(ctx, "test_stats")
	require.NoError(t, err)

	assert.Equal(t, "test_stats", stats.TableName)
	assert.Equal(t, int64(100), stats.RowCount)
	assert.Greater(t, stats.TableSizeBytes, int64(0))
	assert.Len(t, stats.Indexes, 2) // PRIMARY KEY + idx_stats_email
}
