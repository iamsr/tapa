package dryrun

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/iamsr/tapa/pkg/models"
)

// Executor executes SQL statements and captures errors
type Executor struct {
	databaseType string
}

// validateIdentifier ensures identifier is safe for SQL interpolation
func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	// Allow alphanumeric and underscore, must start with letter or underscore
	if !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(name) {
		return fmt.Errorf("invalid identifier: %s (must contain only letters, numbers, and underscores)", name)
	}
	return nil
}

// NewExecutor creates a new SQL executor
func NewExecutor(databaseType string) *Executor {
	return &Executor{
		databaseType: databaseType,
	}
}

// ExecuteSQL executes SQL in a transaction and captures errors
func (e *Executor) ExecuteSQL(ctx context.Context, db *sql.DB, sqlText string, schemaName string) *models.DryRunResult {
	startTime := time.Now()

	result := &models.DryRunResult{
		Status:         models.DryRunStatusSuccess,
		TempSchemaName: schemaName,
		RolledBack:     true,
	}

	// Mock mode (no database connection)
	if db == nil {
		return e.mockExecuteSQL(sqlText, result)
	}

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		result.Status = models.DryRunStatusFailed
		result.ErrorCount = 1
		result.Errors = []models.ExecutionError{
			{
				ErrorType: models.ErrorTypeUnknown,
				Message:   fmt.Sprintf("Failed to start transaction: %v", err),
				Severity:  models.SeverityFatal,
			},
		}
		result.ExecutionTimeMS = time.Since(startTime).Milliseconds()
		return result
	}

	// Ensure rollback on function exit
	defer tx.Rollback()

	// Set search path to temp schema (PostgreSQL)
	if e.databaseType == "postgresql" && schemaName != "" {
		// Validate schema name to prevent SQL injection
		if err := validateIdentifier(schemaName); err != nil {
			result.Errors = append(result.Errors, models.ExecutionError{
				ErrorType: models.ErrorTypePermission,
				Message:   fmt.Sprintf("Invalid schema name: %v", err),
				Severity:  models.SeverityFatal,
			})
			result.ErrorCount++
			result.Status = models.DryRunStatusFailed
			result.ExecutionTimeMS = time.Since(startTime).Milliseconds()
			return result
		}

		_, err := tx.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", schemaName))
		if err != nil {
			result.Errors = append(result.Errors, models.ExecutionError{
				ErrorType: models.ErrorTypePermission,
				Message:   fmt.Sprintf("Failed to set search path: %v", err),
				Severity:  models.SeverityError,
			})
			result.ErrorCount++
		}
	}

	// Execute SQL statements
	statements := e.splitStatements(sqlText)
	for i, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		_, err := tx.ExecContext(ctx, stmt)
		if err != nil {
			execError := e.parseError(err, stmt, i+1)
			result.Errors = append(result.Errors, execError)
			result.ErrorCount++
			result.Status = models.DryRunStatusFailed
		}
	}

	result.ExecutionTimeMS = time.Since(startTime).Milliseconds()
	return result
}

// mockExecuteSQL performs syntax validation without database
func (e *Executor) mockExecuteSQL(sqlText string, result *models.DryRunResult) *models.DryRunResult {
	// Basic syntax validation
	sqlLower := strings.ToLower(sqlText)

	// Check for obvious syntax errors
	invalidPatterns := []string{
		"select from where",
		"insert into values",
		"update set where from",
	}

	for _, pattern := range invalidPatterns {
		if strings.Contains(sqlLower, pattern) {
			result.Status = models.DryRunStatusFailed
			result.ErrorCount = 1
			result.Errors = []models.ExecutionError{
				{
					ErrorType: models.ErrorTypeSyntaxError,
					Message:   "SQL syntax error detected",
					SQL:       sqlText,
					Severity:  models.SeverityError,
				},
			}
			return result
		}
	}

	result.Status = models.DryRunStatusSuccess
	return result
}

// splitStatements splits SQL text into individual statements
func (e *Executor) splitStatements(sqlText string) []string {
	// Simple split by semicolon (doesn't handle strings/comments perfectly)
	statements := strings.Split(sqlText, ";")

	var result []string
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// parseError converts database error to ExecutionError
func (e *Executor) parseError(err error, sql string, lineNumber int) models.ExecutionError {
	errMsg := err.Error()
	errLower := strings.ToLower(errMsg)

	execError := models.ExecutionError{
		ErrorType:  models.ErrorTypeUnknown,
		Message:    errMsg,
		SQL:        sql,
		LineNumber: lineNumber,
		Severity:   models.SeverityError,
	}

	// Classify error type based on message
	switch {
	case strings.Contains(errLower, "constraint"):
		execError.ErrorType = models.ErrorTypeConstraintViolation
	case strings.Contains(errLower, "foreign key"):
		execError.ErrorType = models.ErrorTypeConstraintViolation
	case strings.Contains(errLower, "syntax"):
		execError.ErrorType = models.ErrorTypeSyntaxError
	case strings.Contains(errLower, "type"):
		execError.ErrorType = models.ErrorTypeTypeConversion
	case strings.Contains(errLower, "permission") || strings.Contains(errLower, "denied"):
		execError.ErrorType = models.ErrorTypePermission
	case strings.Contains(errLower, "deadlock"):
		execError.ErrorType = models.ErrorTypeDeadlock
	case strings.Contains(errLower, "disk") || strings.Contains(errLower, "space"):
		execError.ErrorType = models.ErrorTypeResourceExhaustion
	}

	// Extract SQL state for PostgreSQL
	if e.databaseType == "postgresql" {
		// PostgreSQL errors have format: "pq: error message (SQLSTATE xxxxx)"
		if idx := strings.Index(errMsg, "SQLSTATE"); idx != -1 {
			if idx+14 <= len(errMsg) {
				execError.SQLState = errMsg[idx+9 : idx+14]
			}
		}
	}

	return execError
}
