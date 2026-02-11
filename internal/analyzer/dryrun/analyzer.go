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
	sourceSchema  string // Source schema to clone from (e.g., "public" for PostgreSQL)
	schemaCreator *db.TempSchemaCreator
	executor      *Executor
}

// NewAnalyzer creates a new dry-run analyzer
func NewAnalyzer(databaseType string, database *sql.DB) *Analyzer {
	// Determine source schema based on database type
	sourceSchema := "public" // PostgreSQL default
	if databaseType == "mysql" {
		// MySQL doesn't have "public" schema concept - use database name
		// In MySQL, schema == database, so we'll use the current database
		sourceSchema = "" // Will be determined from connection if needed
	}

	return &Analyzer{
		databaseType:  databaseType,
		db:            database,
		sourceSchema:  sourceSchema,
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
	var cloneErr error
	if op.TableName != "" {
		cloneErr = a.schemaCreator.CloneSchema(ctx, a.db, a.sourceSchema, schemaName, []string{op.TableName})
	}

	// Execute SQL in temp schema
	result := a.executor.ExecuteSQL(ctx, a.db, op.SQL, schemaName)

	// If clone failed, add warning to result
	if cloneErr != nil {
		if result.Warnings == nil {
			result.Warnings = []models.ExecutionWarning{}
		}
		result.Warnings = append(result.Warnings, models.ExecutionWarning{
			WarningType: "SCHEMA_CLONE_FAILED",
			Message:     fmt.Sprintf("Failed to clone table structure: %v", cloneErr),
			Suggestion:  "Dry-run executed on empty schema. Constraints may not be validated.",
		})
		result.WarningCount++
	}

	// Attach result to operation
	op.DryRunResult = result

	return nil
}

// AnalyzeMigration performs dry-run for all operations in a migration
func (a *Analyzer) AnalyzeMigration(ctx context.Context, migration *models.Migration) error {
	for _, op := range migration.Operations {
		if op == nil {
			continue // Skip nil operations
		}
		if err := a.AnalyzeOperation(ctx, op); err != nil {
			return err
		}
	}
	return nil
}
