package db

import (
	"context"
)

// TableStats contains statistical information about a table
type TableStats struct {
	TableName      string
	RowCount       int64
	TableSizeBytes int64
	IndexSizeBytes int64
	Indexes        []IndexInfo
}

// IndexInfo contains information about an index
type IndexInfo struct {
	IndexName string
	Columns   []string
	IndexType string
	IsUnique  bool
}

// ForeignKeyInfo contains information about a foreign key constraint
type ForeignKeyInfo struct {
	Name             string
	Column           string
	ReferencedTable  string
	ReferencedColumn string
}

// Introspector provides database introspection capabilities
type Introspector interface {
	// Connect establishes a connection to the database
	Connect(ctx context.Context) error

	// Close closes the database connection
	Close() error

	// GetTableStats retrieves statistics for a table
	GetTableStats(ctx context.Context, tableName string) (*TableStats, error)

	// TableExists checks if a table exists
	TableExists(ctx context.Context, tableName string) (bool, error)
}
