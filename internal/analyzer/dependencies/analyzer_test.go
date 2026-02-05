package dependencies

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/dma/internal/db"
	"github.com/yourusername/dma/pkg/models"
)

func TestGetDependencyAnalyzer(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{"postgresql", "postgresql", false},
		{"mysql", "mysql", true},
		{"unsupported", "oracle", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer, err := GetDependencyAnalyzer(tt.dbType, nil)
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

func TestPostgresDependencyAnalyzer_FindIndexDependencies_DropTable(t *testing.T) {
	// Mock introspector that returns a table with indexes
	mockIntr := &mockIntrospector{
		tableStats: &db.TableStats{
			TableName: "users",
			Indexes: []db.IndexInfo{
				{
					IndexName: "idx_users_email",
					Columns:   []string{"email"},
					IndexType: "btree",
					IsUnique:  true,
				},
				{
					IndexName: "idx_users_created_at",
					Columns:   []string{"created_at"},
					IndexType: "btree",
					IsUnique:  false,
				},
			},
		},
	}

	analyzer := newPostgresDependencyAnalyzer(mockIntr)

	op := &models.Operation{
		Type:      models.OperationTypeDropTable,
		TableName: "users",
		SQL:       "DROP TABLE users",
	}

	deps, err := analyzer.FindDependencies(context.Background(), op)
	require.NoError(t, err)

	// Should find both indexes
	assert.Len(t, deps, 2)

	// Verify first index
	assert.Equal(t, models.DependencyTypeIndex, deps[0].Type)
	assert.Equal(t, "idx_users_email", deps[0].Name)
	assert.Equal(t, models.ImpactBreaks, deps[0].ImpactLevel)
	assert.Contains(t, deps[0].Description, "idx_users_email")

	// Verify second index
	assert.Equal(t, models.DependencyTypeIndex, deps[1].Type)
	assert.Equal(t, "idx_users_created_at", deps[1].Name)
	assert.Equal(t, models.ImpactBreaks, deps[1].ImpactLevel)
}

func TestPostgresDependencyAnalyzer_FindIndexDependencies_DropColumn(t *testing.T) {
	// Mock introspector - not needed for this test as column detection not yet supported
	mockIntr := &mockIntrospector{}

	analyzer := newPostgresDependencyAnalyzer(mockIntr)

	op := &models.Operation{
		Type:      models.OperationTypeDropColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users DROP COLUMN email",
	}

	deps, err := analyzer.FindDependencies(context.Background(), op)
	require.NoError(t, err)

	// Should return empty - column-level index detection not yet implemented
	// TODO: This will be implemented when we add direct SQL query support
	assert.Len(t, deps, 0)
}

func TestPostgresDependencyAnalyzer_FindIndexDependencies_AlterColumn(t *testing.T) {
	mockIntr := &mockIntrospector{}

	analyzer := newPostgresDependencyAnalyzer(mockIntr)

	op := &models.Operation{
		Type:      models.OperationTypeAlterColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ALTER COLUMN email TYPE text",
	}

	deps, err := analyzer.FindDependencies(context.Background(), op)
	require.NoError(t, err)

	// Should return empty - column-level index detection not yet implemented
	assert.Len(t, deps, 0)
}

func TestPostgresDependencyAnalyzer_FindDependencies_NoIntrospector(t *testing.T) {
	analyzer := newPostgresDependencyAnalyzer(nil)

	op := &models.Operation{
		Type:      models.OperationTypeDropTable,
		TableName: "users",
		SQL:       "DROP TABLE users",
	}

	deps, err := analyzer.FindDependencies(context.Background(), op)
	require.NoError(t, err)

	// Should return empty when no introspector available
	assert.Len(t, deps, 0)
}

func TestPostgresDependencyAnalyzer_FindDependencies_NonBreakingOperation(t *testing.T) {
	mockIntr := &mockIntrospector{}
	analyzer := newPostgresDependencyAnalyzer(mockIntr)

	op := &models.Operation{
		Type:      models.OperationTypeCreateTable,
		TableName: "users",
		SQL:       "CREATE TABLE users (id int)",
	}

	deps, err := analyzer.FindDependencies(context.Background(), op)
	require.NoError(t, err)

	// CREATE TABLE doesn't affect existing dependencies
	assert.Len(t, deps, 0)
}

// mockIntrospector implements db.Introspector for testing
type mockIntrospector struct {
	tableStats *db.TableStats
	tableError error
}

func (m *mockIntrospector) Connect(ctx context.Context) error {
	return nil
}

func (m *mockIntrospector) Close() error {
	return nil
}

func (m *mockIntrospector) TableExists(ctx context.Context, name string) (bool, error) {
	return true, nil
}

func (m *mockIntrospector) GetTableStats(ctx context.Context, name string) (*db.TableStats, error) {
	if m.tableError != nil {
		return nil, m.tableError
	}
	if m.tableStats != nil {
		return m.tableStats, nil
	}
	return &db.TableStats{TableName: name}, nil
}
