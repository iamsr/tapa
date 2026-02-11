package dryrun

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/pkg/models"
)

// Analyzer performs dry-run analysis by executing migrations in temp schemas
type Analyzer struct {
	databaseType  string
	db            *sql.DB
	schemaCreator *db.TempSchemaCreator
	executor      *Executor
}

// NewAnalyzer creates a new dry-run analyzer
func NewAnalyzer(databaseType string, database *sql.DB) *Analyzer {
	return &Analyzer{
		databaseType:  databaseType,
		db:            database,
		schemaCreator: db.NewTempSchemaCreator(databaseType),
		executor:      NewExecutor(databaseType),
	}
}

// AnalyzeOperation performs dry-run execution for a single operation
func (a *Analyzer) AnalyzeOperation(ctx context.Context, op *models.Operation) error {
	// Skip if no database connection
	if a.db == nil {
		op.DryRunResult = &models.DryRunResult{
			Status:     models.DryRunStatusSkipped,
			RolledBack: false,
		}
		return nil
	}

	// Create temporary schema
	schemaName, cleanup, err := a.schemaCreator.CreateSchema(ctx, a.db)
	if err != nil {
		return fmt.Errorf("failed to create temp schema: %w", err)
	}
	defer cleanup(ctx)

	// Clone relevant table structures if needed
	if op.TableName != "" {
		err = a.schemaCreator.CloneSchema(ctx, a.db, "public", schemaName, []string{op.TableName})
		if err != nil {
			// Non-fatal: continue with empty schema
		}
	}

	// Execute SQL in temp schema
	result := a.executor.ExecuteSQL(ctx, a.db, op.SQL, schemaName)

	// Attach result to operation
	op.DryRunResult = result

	return nil
}

// AnalyzeMigration performs dry-run for all operations in a migration
func (a *Analyzer) AnalyzeMigration(ctx context.Context, migration *models.Migration) error {
	for _, op := range migration.Operations {
		if err := a.AnalyzeOperation(ctx, op); err != nil {
			return err
		}
	}
	return nil
}
