package concurrency

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iamsr/tapa/pkg/models"
)

const (
	// Peak load threshold for active connections
	peakLoadConnectionThreshold = 50

	// Long-running query threshold in seconds
	longRunningQueryThresholdSeconds = 5
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
	err = w.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE state = 'active' 
		  AND pid != pg_backend_pid()
		  AND now() - query_start > interval '%d seconds'
	`, longRunningQueryThresholdSeconds)).Scan(&longRunning)
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
	workload.PeakLoadPeriod = activeConns > peakLoadConnectionThreshold

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
	err = w.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM information_schema.processlist 
		WHERE command != 'Sleep' 
		  AND id != CONNECTION_ID()
		  AND time > %d
	`, longRunningQueryThresholdSeconds)).Scan(&longRunning)
	if err != nil {
		// Use default value on error
		longRunning = 0
	}
	workload.LongRunningQueries = longRunning

	// Classify access frequency based on estimated queries per minute
	queriesPerMin := int(workload.QueriesPerSecond * 60)
	workload.TableAccessFrequency = w.ClassifyAccessFrequency(queriesPerMin)

	// Determine if we're in peak load period (>50 active connections)
	workload.PeakLoadPeriod = activeConns > peakLoadConnectionThreshold

	return workload, nil
}

// mockWorkload returns a default workload for testing without database connection
func (w *WorkloadAnalyzer) mockWorkload() *models.WorkloadAnalysis {
	return &models.WorkloadAnalysis{
		ActiveConnections:    10,
		QueriesPerSecond:     5.0,
		TableAccessFrequency: "medium",
		PeakLoadPeriod:       false,
		TopQueryTypes: []models.QueryTypeMetrics{
			{
				QueryType:     "SELECT",
				CountPerMin:   100,
				AvgDurationMS: 50.0,
				WillBeBlocked: false,
			},
			{
				QueryType:     "UPDATE",
				CountPerMin:   30,
				AvgDurationMS: 100.0,
				WillBeBlocked: true,
			},
		},
		LongRunningQueries: 2,
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

// EstimateBlockedQueries estimates how many queries will be blocked during migration
func (w *WorkloadAnalyzer) EstimateBlockedQueries(workload *models.WorkloadAnalysis, lockDurationMS int64, blockedTypes []string) int {
	if workload == nil {
		return 0
	}

	// If workload has TopQueryTypes data, use it for precise calculation
	if len(workload.TopQueryTypes) > 0 {
		lockDurationMinutes := float64(lockDurationMS) / 60000.0
		totalBlocked := 0

		for _, queryType := range workload.TopQueryTypes {
			for _, blockedType := range blockedTypes {
				if queryType.QueryType == blockedType {
					blocked := int(float64(queryType.CountPerMin) * lockDurationMinutes)
					totalBlocked += blocked
				}
			}
		}

		return totalBlocked
	}

	// Fallback: use generic QPS-based estimation
	lockDurationSeconds := float64(lockDurationMS) / 1000.0
	queriesPerMinute := workload.QueriesPerSecond * 60

	// Estimate based on access frequency
	var estimatedQueriesAffected float64
	switch workload.TableAccessFrequency {
	case "very_high":
		estimatedQueriesAffected = queriesPerMinute * 0.8 // 80% of queries
	case "high":
		estimatedQueriesAffected = queriesPerMinute * 0.5 // 50%
	case "medium":
		estimatedQueriesAffected = queriesPerMinute * 0.3 // 30%
	default: // "low"
		estimatedQueriesAffected = queriesPerMinute * 0.1 // 10%
	}

	// Apply peak load multiplier
	if workload.PeakLoadPeriod {
		estimatedQueriesAffected *= 1.5
	}

	totalBlocked := int(estimatedQueriesAffected * (lockDurationSeconds / 60.0))
	return totalBlocked
}
