package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/iamsr/tapa/internal/db"
)

// Introspector queries MySQL database for schema information
type Introspector struct {
	connURL string
	db      *sql.DB
}

// NewIntrospector creates a new MySQL introspector
func NewIntrospector(connURL string) *Introspector {
	return &Introspector{
		connURL: connURL,
	}
}

// Connect establishes a connection to the database
func (i *Introspector) Connect(ctx context.Context) error {
	db, err := sql.Open("mysql", i.connURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	i.db = db
	return nil
}

// Close closes the database connection
func (i *Introspector) Close() error {
	if i.db != nil {
		return i.db.Close()
	}
	return nil
}

// TableExists checks if a table exists
func (i *Introspector) TableExists(ctx context.Context, tableName string) (bool, error) {
	if i.db == nil {
		return false, fmt.Errorf("no database connection")
	}

	query := `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name = ?
	`

	var count int
	err := i.db.QueryRowContext(ctx, query, tableName).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check table existence: %w", err)
	}

	return count > 0, nil
}

// GetTableStats retrieves statistics for a table
func (i *Introspector) GetTableStats(ctx context.Context, tableName string) (*db.TableStats, error) {
	if i.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	stats := &db.TableStats{
		TableName: tableName,
	}

	// Query information_schema.tables for table statistics
	query := `
		SELECT
			COALESCE(table_rows, 0) AS row_count,
			COALESCE(data_length, 0) AS data_length,
			COALESCE(index_length, 0) AS index_length
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name = ?
	`

	var dataLength, indexLength int64
	err := i.db.QueryRowContext(ctx, query, tableName).Scan(
		&stats.RowCount,
		&dataLength,
		&indexLength,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}

	// Calculate total size in bytes
	stats.TableSizeBytes = dataLength
	stats.IndexSizeBytes = indexLength

	// Get index information
	indexes, err := i.GetIndexes(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	stats.Indexes = indexes

	return stats, nil
}

// GetIndexes retrieves index information for a table
func (i *Introspector) GetIndexes(ctx context.Context, tableName string) ([]db.IndexInfo, error) {
	if i.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	query := `
		SELECT
			index_name,
			column_name,
			non_unique,
			index_type
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		ORDER BY index_name, seq_in_index
	`

	rows, err := i.db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	// Map to aggregate columns by index name
	indexMap := make(map[string]*db.IndexInfo)

	for rows.Next() {
		var indexName, columnName, indexType string
		var nonUnique int

		if err := rows.Scan(&indexName, &columnName, &nonUnique, &indexType); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}

		if _, exists := indexMap[indexName]; !exists {
			indexMap[indexName] = &db.IndexInfo{
				IndexName: indexName,
				Columns:   make([]string, 0),
				IndexType: indexType,
				IsUnique:  nonUnique == 0,
			}
		}

		indexMap[indexName].Columns = append(indexMap[indexName].Columns, columnName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating index rows: %w", err)
	}

	// Convert map to slice
	indexes := make([]db.IndexInfo, 0, len(indexMap))
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	// Sort by index name for deterministic output
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].IndexName < indexes[j].IndexName
	})

	return indexes, nil
}

// GetForeignKeys retrieves foreign key constraints for a table
func (i *Introspector) GetForeignKeys(ctx context.Context, tableName string) ([]db.ForeignKeyInfo, error) {
	if i.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	query := `
		SELECT
			constraint_name,
			column_name,
			referenced_table_name,
			referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		  AND referenced_table_name IS NOT NULL
		ORDER BY constraint_name, ordinal_position
	`

	rows, err := i.db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []db.ForeignKeyInfo

	for rows.Next() {
		var fk db.ForeignKeyInfo
		if err := rows.Scan(&fk.Name, &fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key row: %w", err)
		}
		fks = append(fks, fk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign key rows: %w", err)
	}

	return fks, nil
}
