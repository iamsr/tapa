package dependencies

import (
	"context"
	"fmt"
	"regexp"

	"github.com/yourusername/dma/internal/db"
	"github.com/yourusername/dma/pkg/models"
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
		return nil, fmt.Errorf("MySQL dependency analyzer not yet implemented")
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
func extractColumnName(sql string) string {
	// Match: DROP COLUMN column_name
	re := regexp.MustCompile(`DROP\s+COLUMN\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return matches[1]
	}

	// Match: ALTER COLUMN column_name
	re = regexp.MustCompile(`ALTER\s+COLUMN\s+(\w+)`)
	matches = re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return matches[1]
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
