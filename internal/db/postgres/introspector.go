package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/iamsr/tapa/internal/db"
)

// Introspector implements database introspection for PostgreSQL
type Introspector struct {
	connURL string
	conn    *pgx.Conn
}

// NewIntrospector creates a new PostgreSQL introspector
func NewIntrospector(connURL string) *Introspector {
	return &Introspector{
		connURL: connURL,
	}
}

// Connect establishes a connection to the database
func (i *Introspector) Connect(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, i.connURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	i.conn = conn
	return nil
}

// Close closes the database connection
func (i *Introspector) Close() error {
	if i.conn != nil {
		return i.conn.Close(context.Background())
	}
	return nil
}

// TableExists checks if a table exists
func (i *Introspector) TableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public'
			AND tablename = $1
		)
	`

	err := i.conn.QueryRow(ctx, query, tableName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check table existence: %w", err)
	}

	return exists, nil
}

// GetTableStats retrieves statistics for a table
func (i *Introspector) GetTableStats(ctx context.Context, tableName string) (*db.TableStats, error) {
	stats := &db.TableStats{
		TableName: tableName,
	}

	// Get row count and size
	query := `
		SELECT
			reltuples::bigint AS row_count,
			pg_total_relation_size(c.oid) AS total_size,
			pg_indexes_size(c.oid) AS indexes_size
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1
		AND n.nspname = 'public'
		AND c.relkind = 'r'
	`

	err := i.conn.QueryRow(ctx, query, tableName).Scan(
		&stats.RowCount,
		&stats.TableSizeBytes,
		&stats.IndexSizeBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}

	// Get index information
	indexQuery := `
		SELECT
			i.relname AS index_name,
			a.attname AS column_name,
			am.amname AS index_type,
			ix.indisunique AS is_unique
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON i.relam = am.oid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $1
		AND t.relkind = 'r'
		ORDER BY i.relname, a.attnum
	`

	rows, err := i.conn.Query(ctx, indexQuery, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get index info: %w", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*db.IndexInfo)

	for rows.Next() {
		var indexName, columnName, indexType string
		var isUnique bool

		err := rows.Scan(&indexName, &columnName, &indexType, &isUnique)
		if err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}

		if _, exists := indexMap[indexName]; !exists {
			indexMap[indexName] = &db.IndexInfo{
				IndexName: indexName,
				IndexType: indexType,
				IsUnique:  isUnique,
				Columns:   make([]string, 0),
			}
		}

		indexMap[indexName].Columns = append(indexMap[indexName].Columns, columnName)
	}

	stats.Indexes = make([]db.IndexInfo, 0, len(indexMap))
	for _, idx := range indexMap {
		stats.Indexes = append(stats.Indexes, *idx)
	}

	return stats, nil
}
