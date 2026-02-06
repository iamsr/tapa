package dependencies

import (
	"context"
	"fmt"
	"regexp"

	"github.com/iamsr/dma/internal/db"
	"github.com/iamsr/dma/pkg/models"
)

// DependencyAnalyzer finds dependencies affected by an operation
type DependencyAnalyzer interface {
	// FindDependencies discovers what depends on the operation's target
	FindDependencies(ctx context.Context, op *models.Operation) ([]models.Dependency, error)
}

// GetDependencyAnalyzer returns appropriate analyzer for database type
func GetDependencyAnalyzer(dbType string, introspector db.Introspector) (DependencyAnalyzer, error) {
	switch dbType {
	case "postgresql":
		return newPostgresDependencyAnalyzer(introspector), nil
	case "mysql":
		return NewMySQLDependencyAnalyzer(introspector), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// postgresDependencyAnalyzer implements DependencyAnalyzer for PostgreSQL
type postgresDependencyAnalyzer struct {
	introspector db.Introspector
}

func newPostgresDependencyAnalyzer(introspector db.Introspector) *postgresDependencyAnalyzer {
	return &postgresDependencyAnalyzer{
		introspector: introspector,
	}
}

// FindDependencies finds dependencies for PostgreSQL
func (a *postgresDependencyAnalyzer) FindDependencies(ctx context.Context, op *models.Operation) ([]models.Dependency, error) {
	deps := []models.Dependency{}

	// Only check dependencies for operations that might affect them
	if op.Type != models.OperationTypeDropColumn &&
		op.Type != models.OperationTypeAlterColumn &&
		op.Type != models.OperationTypeDropTable {
		return deps, nil
	}

	// If no introspector, can't query dependencies
	if a.introspector == nil {
		return deps, nil
	}

	// Find indexes on column for DROP/ALTER COLUMN
	if op.Type == models.OperationTypeDropColumn || op.Type == models.OperationTypeAlterColumn {
		columnName := extractColumnName(op.SQL)
		if columnName != "" {
			indexDeps, err := a.findIndexesOnColumn(ctx, op.TableName, columnName)
			if err != nil {
				return nil, fmt.Errorf("failed to find index dependencies: %w", err)
			}
			deps = append(deps, indexDeps...)
		}
	}

	// Find indexes on table for DROP TABLE
	if op.Type == models.OperationTypeDropTable {
		indexDeps, err := a.findIndexesOnTable(ctx, op.TableName)
		if err != nil {
			return nil, fmt.Errorf("failed to find index dependencies: %w", err)
		}
		deps = append(deps, indexDeps...)
	}

	return deps, nil
}

// extractColumnName tries to extract column name from DROP COLUMN or ALTER COLUMN SQL
// Handles both quoted ("column_name") and unquoted (column_name) identifiers
// Returns empty string if pattern not matched
func extractColumnName(sql string) string {
	// Match DROP COLUMN with case-insensitive flag and support for quoted identifiers
	re := regexp.MustCompile(`(?i)DROP\s+COLUMN\s+(?:"([^"]+)|(\w+))`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		if matches[1] != "" {
			return matches[1] // quoted identifier
		}
		if matches[2] != "" {
			return matches[2] // unquoted identifier
		}
	}

	// Match ALTER COLUMN with case-insensitive flag and support for quoted identifiers
	re = regexp.MustCompile(`(?i)ALTER\s+COLUMN\s+(?:"([^"]+)|(\w+))`)
	matches = re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		if matches[1] != "" {
			return matches[1] // quoted identifier
		}
		if matches[2] != "" {
			return matches[2] // unquoted identifier
		}
	}

	return ""
}

// findIndexesOnColumn finds indexes that depend on a specific column
// TODO: This requires direct SQL access to query pg_index and pg_attribute
// The current introspector interface doesn't support column-level index queries
// For now, returns empty slice. Will be implemented when direct database connection is added.
func (a *postgresDependencyAnalyzer) findIndexesOnColumn(ctx context.Context, tableName, columnName string) ([]models.Dependency, error) {
	return []models.Dependency{}, nil
}

// findIndexesOnTable finds all indexes on a table using GetTableStats
func (a *postgresDependencyAnalyzer) findIndexesOnTable(ctx context.Context, tableName string) ([]models.Dependency, error) {
	stats, err := a.introspector.GetTableStats(ctx, tableName)
	if err != nil {
		return nil, err
	}

	deps := []models.Dependency{}
	for _, idx := range stats.Indexes {
		deps = append(deps, models.Dependency{
			Type:        models.DependencyTypeIndex,
			Name:        idx.IndexName,
			ImpactLevel: models.ImpactBreaks,
			Description: fmt.Sprintf("Index %s will be dropped with table", idx.IndexName),
		})
	}

	return deps, nil
}

// MySQLDependencyAnalyzer finds dependencies in MySQL
type MySQLDependencyAnalyzer struct {
	introspector db.Introspector
}

// NewMySQLDependencyAnalyzer creates a MySQL dependency analyzer
func NewMySQLDependencyAnalyzer(introspector db.Introspector) *MySQLDependencyAnalyzer {
	return &MySQLDependencyAnalyzer{introspector: introspector}
}

// FindDependencies finds what breaks when operation executes
func (a *MySQLDependencyAnalyzer) FindDependencies(ctx context.Context, op *models.Operation) ([]models.Dependency, error) {
	// MySQL dependency detection uses information_schema
	// Similar to PostgreSQL but queries different tables

	if a.introspector == nil {
		return []models.Dependency{}, nil
	}

	switch op.Type {
	case models.OperationTypeDropTable, models.OperationTypeDropColumn:
		return a.findIndexDependencies(ctx, op)
	default:
		return []models.Dependency{}, nil
	}
}

// findIndexDependencies finds indexes that will break
func (a *MySQLDependencyAnalyzer) findIndexDependencies(ctx context.Context, op *models.Operation) ([]models.Dependency, error) {
	stats, err := a.introspector.GetTableStats(ctx, op.TableName)
	if err != nil {
		return nil, err
	}

	var deps []models.Dependency

	for _, idx := range stats.Indexes {
		if op.Type == models.OperationTypeDropColumn {
			// Check if index uses the column
			for _, col := range idx.Columns {
				if col == op.ColumnName {
					deps = append(deps, models.Dependency{
						Type:        models.DependencyTypeIndex,
						Name:        idx.IndexName,
						ImpactLevel: models.ImpactBreaks,
						Description: fmt.Sprintf("Index '%s' depends on column '%s'", idx.IndexName, op.ColumnName),
					})
					break
				}
			}
		} else if op.Type == models.OperationTypeDropTable {
			deps = append(deps, models.Dependency{
				Type:        models.DependencyTypeIndex,
				Name:        idx.IndexName,
				ImpactLevel: models.ImpactBreaks,
				Description: fmt.Sprintf("Index '%s' will be dropped with table", idx.IndexName),
			})
		}
	}

	return deps, nil
}
