package models

import (
	"fmt"
)

// DryRunStatus represents the overall status of a dry-run execution
type DryRunStatus string

const (
	DryRunStatusSuccess  DryRunStatus = "SUCCESS"
	DryRunStatusFailed   DryRunStatus = "FAILED"
	DryRunStatusSkipped  DryRunStatus = "SKIPPED" // When dry-run not possible
	DryRunStatusTimedOut DryRunStatus = "TIMED_OUT"
)

// ErrorType categorizes different types of execution errors
type ErrorType string

const (
	ErrorTypeConstraintViolation ErrorType = "CONSTRAINT_VIOLATION"
	ErrorTypeSyntaxError         ErrorType = "SYNTAX_ERROR"
	ErrorTypeTypeConversion      ErrorType = "TYPE_CONVERSION"
	ErrorTypePermission          ErrorType = "PERMISSION_DENIED"
	ErrorTypeResourceExhaustion  ErrorType = "RESOURCE_EXHAUSTION"
	ErrorTypeDeadlock            ErrorType = "DEADLOCK"
	ErrorTypeUnknown             ErrorType = "UNKNOWN"
)

// ErrorSeverity represents the severity level of an execution error
type ErrorSeverity string

const (
	SeverityError ErrorSeverity = "ERROR"
	SeverityFatal ErrorSeverity = "FATAL"
)

// DryRunResult represents the result of executing a migration in dry-run mode.
//
// Invariants that must be maintained by callers:
//   - ErrorCount should match len(Errors)
//   - WarningCount should match len(Warnings)
//   - Status should be FAILED if ErrorCount > 0
//   - Status should be SUCCESS only if ErrorCount == 0
type DryRunResult struct {
	Status          DryRunStatus       `json:"status"`
	ExecutionTimeMS int64              `json:"execution_time_ms"`
	ErrorCount      int                `json:"error_count"`
	WarningCount    int                `json:"warning_count"`
	Errors          []ExecutionError   `json:"errors,omitempty"`
	Warnings        []ExecutionWarning `json:"warnings,omitempty"`
	TempSchemaName  string             `json:"temp_schema_name,omitempty"`
	RolledBack      bool               `json:"rolled_back"`
}

// ExecutionError represents a runtime error during migration execution
type ExecutionError struct {
	ErrorType  ErrorType     `json:"error_type"`
	Message    string        `json:"message"`
	SQL        string        `json:"sql,omitempty"`
	LineNumber int           `json:"line_number,omitempty"`
	Details    string        `json:"details,omitempty"`
	Severity   ErrorSeverity `json:"severity"`            // "ERROR", "FATAL"
	SQLState   string        `json:"sql_state,omitempty"` // PostgreSQL error code
}

// ExecutionWarning represents non-fatal issues detected during execution
type ExecutionWarning struct {
	WarningType string `json:"warning_type"`
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// IsSuccessful returns true if dry-run completed without errors
func (r *DryRunResult) IsSuccessful() bool {
	return r.Status == DryRunStatusSuccess && r.ErrorCount == 0
}

// HasErrors returns true if any errors were encountered
func (r *DryRunResult) HasErrors() bool {
	return r.ErrorCount > 0
}

// String returns a string representation of the execution error
func (e *ExecutionError) String() string {
	return fmt.Sprintf("[%s] %s", e.ErrorType, e.Message)
}
