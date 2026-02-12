package concurrency

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iamsr/tapa/pkg/models"
)

// WorkloadAnalyzer analyzes database workload patterns
type WorkloadAnalyzer struct {
	databaseType string
	db           *sql.DB
}

// NewWorkloadAnalyzer creates a new workload analyzer
func NewWorkloadAnalyzer(databaseType string, db *sql.DB) *WorkloadAnalyzer {
	return &WorkloadAnalyzer{
		databaseType: databaseType,
		db:           db,
	}
}

// AnalyzeWorkload analyzes the current database workload for a specific table
func (w *WorkloadAnalyzer) AnalyzeWorkload(ctx context.Context, tableName string) (*models.WorkloadAnalysis, error) {
	// If no database connection, return mock workload for testing
	if w.db == nil {
		return w.mockWorkload(), nil
	}

	// Switch on database type to call appropriate analyzer
	switch w.databaseType {
	case "postgresql", "postgres":
		return w.analyzePostgreSQLWorkload(ctx, tableName)
	case "mysql":
		return w.analyzeMySQLWorkload(ctx, tableName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", w.databaseType)
	}
}

// analyzePostgreSQLWorkload queries PostgreSQL system tables for workload information
func (w *WorkloadAnalyzer) analyzePostgreSQLWorkload(ctx context.Context, tableName string) (*models.WorkloadAnalysis, error) {
	workload := &models.WorkloadAnalysis{
		ActiveConnections:    0,
		QueriesPerSecond:     0.0,
		TableAccessFrequency: "low",
		PeakLoadPeriod:       false,
		TopQueryTypes:        []models.QueryTypeMetrics{},
		LongRunningQueries:   0,
	}

	// Query 1: Active connections from pg_stat_activity
	var activeConns int
	err := w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE state = 'active' AND pid != pg_backend_pid()
	`).Scan(&activeConns)
	if err != nil {
		// Use default value on error
		activeConns = 10
	}
	workload.ActiveConnections = activeConns

	// Query 2: Long-running queries (>5 seconds)
	var longRunning int
	err = w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE state = 'active' 
		  AND pid != pg_backend_pid()
		  AND now() - query_start > interval '5 seconds'
	`).Scan(&longRunning)
	if err != nil {
		// Use default value on error
		longRunning = 0
	}
	workload.LongRunningQueries = longRunning

	// Query 3: Try to get query rate from pg_stat_statements if available
	// This extension may not be enabled, so we handle errors gracefully
	var totalCalls float64
	err = w.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(calls), 0) 
		FROM pg_stat_statements
	`).Scan(&totalCalls)
	if err != nil {
		// pg_stat_statements not available or error occurred
		// Use conservative estimate based on active connections
		workload.QueriesPerSecond = float64(activeConns) * 0.5
	} else {
		// Estimate QPS (this is cumulative, so we use a conservative estimate)
		// In production, you'd track this over time
		workload.QueriesPerSecond = 10.0
	}

	// Classify access frequency based on estimated queries per minute
	queriesPerMin := int(workload.QueriesPerSecond * 60)
	workload.TableAccessFrequency = w.ClassifyAccessFrequency(queriesPerMin)

	// Determine if we're in peak load period (>50 active connections)
	workload.PeakLoadPeriod = activeConns > 50

	return workload, nil
}

// analyzeMySQLWorkload queries MySQL system tables for workload information
func (w *WorkloadAnalyzer) analyzeMySQLWorkload(ctx context.Context, tableName string) (*models.WorkloadAnalysis, error) {
	workload := &models.WorkloadAnalysis{
		ActiveConnections:    0,
		QueriesPerSecond:     10.0, // Conservative estimate
		TableAccessFrequency: "low",
		PeakLoadPeriod:       false,
		TopQueryTypes:        []models.QueryTypeMetrics{},
		LongRunningQueries:   0,
	}

	// Query 1: Active connections from information_schema.processlist
	var activeConns int
	err := w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM information_schema.processlist 
		WHERE command != 'Sleep' AND id != CONNECTION_ID()
	`).Scan(&activeConns)
	if err != nil {
		// Use default value on error
		activeConns = 10
	}
	workload.ActiveConnections = activeConns

	// Query 2: Long-running queries (>5 seconds)
	var longRunning int
	err = w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM information_schema.processlist 
		WHERE command != 'Sleep' 
		  AND id != CONNECTION_ID()
		  AND time > 5
	`).Scan(&longRunning)
	if err != nil {
		// Use default value on error
		longRunning = 0
	}
	workload.LongRunningQueries = longRunning

	// Classify access frequency based on estimated queries per minute
	queriesPerMin := int(workload.QueriesPerSecond * 60)
	workload.TableAccessFrequency = w.ClassifyAccessFrequency(queriesPerMin)

	// Determine if we're in peak load period (>50 active connections)
	workload.PeakLoadPeriod = activeConns > 50

	return workload, nil
}

// mockWorkload returns a default workload for testing without database connection
func (w *WorkloadAnalyzer) mockWorkload() *models.WorkloadAnalysis {
	return &models.WorkloadAnalysis{
		ActiveConnections:    10,
		QueriesPerSecond:     15.0,
		TableAccessFrequency: "medium",
		PeakLoadPeriod:       false,
		TopQueryTypes:        []models.QueryTypeMetrics{},
		LongRunningQueries:   0,
	}
}

// ClassifyAccessFrequency classifies table access frequency based on queries per minute
func (w *WorkloadAnalyzer) ClassifyAccessFrequency(queriesPerMin int) string {
	switch {
	case queriesPerMin < 60:
		return "low"
	case queriesPerMin < 300:
		return "medium"
	case queriesPerMin < 1000:
		return "high"
	default:
		return "very_high"
	}
}

// EstimateBlockedQueries estimates the number of queries that will be blocked
func (w *WorkloadAnalyzer) EstimateBlockedQueries(workload *models.WorkloadAnalysis, lockDurationMS int64, blockedTypes []string) int {
	if workload == nil {
		return 0
	}

	// Calculate expected queries during lock period
	lockDurationSec := float64(lockDurationMS) / 1000.0
	expectedQueries := workload.QueriesPerSecond * lockDurationSec

	// Apply multiplier based on access frequency
	var frequencyMultiplier float64
	switch workload.TableAccessFrequency {
	case "very_high":
		frequencyMultiplier = 1.5
	case "high":
		frequencyMultiplier = 1.2
	case "medium":
		frequencyMultiplier = 1.0
	case "low":
		frequencyMultiplier = 0.5
	default:
		frequencyMultiplier = 1.0
	}

	// Apply peak load multiplier
	if workload.PeakLoadPeriod {
		frequencyMultiplier *= 1.3
	}

	// Estimate blocked queries based on blocked types
	// Assume blocked types represent % of total queries
	blockedPercentage := float64(len(blockedTypes)) / 4.0 // 4 main query types (SELECT, INSERT, UPDATE, DELETE)

	estimatedBlocked := int(expectedQueries * frequencyMultiplier * blockedPercentage)

	// Ensure at least 1 if there are any queries and lock duration > 0
	if estimatedBlocked == 0 && lockDurationMS > 0 && workload.QueriesPerSecond > 0 {
		estimatedBlocked = 1
	}

	return estimatedBlocked
}
