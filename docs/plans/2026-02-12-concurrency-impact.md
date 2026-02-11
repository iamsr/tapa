# Concurrency Impact Analysis Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement concurrency impact analysis feature that predicts how migrations affect concurrent database queries, estimates blocked query time, and suggests concurrency-safe alternatives.

**Architecture:** Analyzes lock types, lock duration predictions, and query patterns to estimate concurrency impact. Uses statistical models based on table size, operation type, and database load indicators. Integrates with existing analyzer via `--concurrency` flag (or included in `--comprehensive`).

**Tech Stack:** Go 1.21+, PostgreSQL (pg_stat_activity, pg_locks), MySQL (performance_schema), existing TAPA parser/analyzer infrastructure

---

## Context

Concurrency Impact Analysis is the 5th and final advanced feature. It builds on the foundation of all 4 completed features (Disk Space, Rollback, Data Migration, and Dry-Run). This feature helps predict how migrations will affect concurrent database operations:

- Lock acquisition impact on active queries
- Query blocking and wait time estimates
- Table-level vs row-level locking strategies
- Concurrent DDL alternatives (e.g., CREATE INDEX CONCURRENTLY)
- Workload-specific recommendations based on query patterns

**Dependencies:**
- Requires database connection for querying system tables
- Uses lock type information from existing analyzer
- Builds on Operation models
- May optionally use query logs for workload analysis

**Implementation Strategy:**
1. Create models for concurrency analysis results
2. Implement lock impact calculator
3. Build query pattern analyzer (optional, uses pg_stat_statements)
4. Add concurrency analyzer
5. Integrate into CLI with `--concurrency` flag
6. Add output formatters
7. Write comprehensive tests

---

## Task 1: Concurrency Models

**Goal:** Create data structures for concurrency impact analysis

**Files:**
- Create: `pkg/models/concurrency.go`
- Create: `tests/unit/models/concurrency_test.go`

### Step 1: Write failing test for ConcurrencyAnalysis model

Create test file:

```go
package models_test

import (
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestConcurrencyAnalysis_IsHighImpact(t *testing.T) {
	tests := []struct {
		name           string
		impactScore    int
		want           bool
	}{
		{"low impact", 20, false},
		{"medium impact", 45, false},
		{"high impact", 70, true},
		{"critical impact", 95, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.ConcurrencyAnalysis{
				ImpactScore: tt.impactScore,
			}
			if got := analysis.IsHighImpact(); got != tt.want {
				t.Errorf("IsHighImpact() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcurrencyAnalysis_ImpactLevel(t *testing.T) {
	tests := []struct {
		name        string
		impactScore int
		want        models.ConcurrencyImpactLevel
	}{
		{"minimal", 10, models.ConcurrencyImpactMinimal},
		{"low", 30, models.ConcurrencyImpactLow},
		{"medium", 55, models.ConcurrencyImpactMedium},
		{"high", 75, models.ConcurrencyImpactHigh},
		{"critical", 95, models.ConcurrencyImpactCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.ConcurrencyAnalysis{
				ImpactScore: tt.impactScore,
			}
			if got := analysis.ImpactLevel(); got != tt.want {
				t.Errorf("ImpactLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLockImpact_String(t *testing.T) {
	impact := &models.LockImpact{
		LockType:              models.LockTypeAccessExclusive,
		EstimatedDurationMS:   5000,
		BlockedQueryTypes:     []string{"SELECT", "INSERT", "UPDATE"},
		EstimatedBlockedCount: 25,
		WaitTimeRange:         "2-5 seconds",
	}

	str := impact.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./tests/unit/models -v -run TestConcurrency`
Expected: FAIL (undefined types)

### Step 3: Implement concurrency models

Create `pkg/models/concurrency.go`:

```go
package models

import "fmt"

// ConcurrencyImpactLevel represents the severity of concurrency impact
type ConcurrencyImpactLevel string

const (
	ConcurrencyImpactMinimal  ConcurrencyImpactLevel = "MINIMAL"  // 0-20
	ConcurrencyImpactLow      ConcurrencyImpactLevel = "LOW"      // 21-40
	ConcurrencyImpactMedium   ConcurrencyImpactLevel = "MEDIUM"   // 41-60
	ConcurrencyImpactHigh     ConcurrencyImpactLevel = "HIGH"     // 61-80
	ConcurrencyImpactCritical ConcurrencyImpactLevel = "CRITICAL" // 81-100
)

// ConcurrencyAnalysis represents concurrency impact analysis for a migration
type ConcurrencyAnalysis struct {
	ImpactScore           int                        `json:"impact_score"` // 0-100
	LockImpact            *LockImpact                `json:"lock_impact"`
	WorkloadAnalysis      *WorkloadAnalysis          `json:"workload_analysis,omitempty"`
	SaferAlternatives     []ConcurrentAlternative    `json:"safer_alternatives,omitempty"`
	Recommendations       []string                   `json:"recommendations"`
	EstimatedDowntimeMS   int64                      `json:"estimated_downtime_ms,omitempty"`
	ConcurrencySafe       bool                       `json:"concurrency_safe"`
}

// LockImpact describes the locking behavior and impact
type LockImpact struct {
	LockType              LockType `json:"lock_type"`
	EstimatedDurationMS   int64    `json:"estimated_duration_ms"`
	BlocksReads           bool     `json:"blocks_reads"`
	BlocksWrites          bool     `json:"blocks_writes"`
	BlockedQueryTypes     []string `json:"blocked_query_types"`     // ["SELECT", "INSERT", "UPDATE", "DELETE"]
	EstimatedBlockedCount int      `json:"estimated_blocked_count"` // Number of queries expected to be blocked
	WaitTimeRange         string   `json:"wait_time_range"`         // "0-2 seconds", "5-30 seconds"
	LockAcquisitionRisk   string   `json:"lock_acquisition_risk"`   // "low", "medium", "high"
}

// WorkloadAnalysis describes current database workload patterns
type WorkloadAnalysis struct {
	ActiveConnections    int                `json:"active_connections"`
	QueriesPerSecond     float64            `json:"queries_per_second"`
	TableAccessFrequency string             `json:"table_access_frequency"` // "low", "medium", "high", "very_high"
	PeakLoadPeriod       bool               `json:"peak_load_period"`
	TopQueryTypes        []QueryTypeMetrics `json:"top_query_types"`
	LongRunningQueries   int                `json:"long_running_queries"` // Count of queries > 5 seconds
}

// QueryTypeMetrics describes frequency of different query types
type QueryTypeMetrics struct {
	QueryType    string  `json:"query_type"`    // "SELECT", "INSERT", "UPDATE", "DELETE"
	CountPerMin  int     `json:"count_per_min"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
	WillBeBlocked bool   `json:"will_be_blocked"`
}

// ConcurrentAlternative suggests safer concurrency approaches
type ConcurrentAlternative struct {
	Description       string   `json:"description"`
	LockType          LockType `json:"lock_type"`
	ImpactReduction   int      `json:"impact_reduction"` // Percentage reduction in impact score
	RequiresFeature   string   `json:"requires_feature,omitempty"` // e.g., "PostgreSQL 11+", "ALGORITHM=INPLACE"
	Steps             []string `json:"steps,omitempty"`
	EstimatedTimeMS   int64    `json:"estimated_time_ms"`
	Tradeoffs         []string `json:"tradeoffs,omitempty"`
}

// IsHighImpact returns true if impact score >= 61
func (c *ConcurrencyAnalysis) IsHighImpact() bool {
	return c.ImpactScore >= 61
}

// ImpactLevel returns the impact category based on score
func (c *ConcurrencyAnalysis) ImpactLevel() ConcurrencyImpactLevel {
	switch {
	case c.ImpactScore >= 81:
		return ConcurrencyImpactCritical
	case c.ImpactScore >= 61:
		return ConcurrencyImpactHigh
	case c.ImpactScore >= 41:
		return ConcurrencyImpactMedium
	case c.ImpactScore >= 21:
		return ConcurrencyImpactLow
	default:
		return ConcurrencyImpactMinimal
	}
}

// String returns a string representation of lock impact
func (l *LockImpact) String() string {
	return fmt.Sprintf("%s lock for %dms, blocks %d queries", l.LockType, l.EstimatedDurationMS, l.EstimatedBlockedCount)
}

// BlocksAllWrites returns true if lock blocks all write operations
func (l *LockImpact) BlocksAllWrites() bool {
	return l.LockType == LockTypeAccessExclusive || l.LockType == LockTypeExclusive
}

// BlocksAllReads returns true if lock blocks all read operations
func (l *LockImpact) BlocksAllReads() bool {
	return l.LockType == LockTypeAccessExclusive
}
```

### Step 4: Run test to verify it passes

Run: `go test ./tests/unit/models -v -run TestConcurrency`
Expected: PASS

### Step 5: Commit

```bash
git add pkg/models/concurrency.go tests/unit/models/concurrency_test.go
git commit -m "feat(concurrency): add concurrency analysis models"
```

---

## Task 2: Lock Impact Calculator

**Goal:** Calculate lock impact based on operation type and table characteristics

**Files:**
- Create: `internal/analyzer/concurrency/lock_calculator.go`
- Create: `internal/analyzer/concurrency/lock_calculator_test.go`

### Step 1: Write failing test for lock calculator

Create test file:

```go
package concurrency_test

import (
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/pkg/models"
)

func TestLockCalculator_CalculateImpact(t *testing.T) {
	calculator := concurrency.NewLockCalculator("postgresql")

	tests := []struct {
		name          string
		op            *models.Operation
		wantBlocksAll bool
	}{
		{
			name: "ADD COLUMN with default blocks all",
			op: &models.Operation{
				Type:            models.OperationTypeAddColumn,
				TableName:       "users",
				RequiresRewrite: true,
				LockType:        models.LockTypeAccessExclusive,
				RowCount:        1000000,
			},
			wantBlocksAll: true,
		},
		{
			name: "CREATE INDEX CONCURRENTLY low impact",
			op: &models.Operation{
				Type:            models.OperationTypeCreateIndex,
				TableName:       "orders",
				RequiresRewrite: false,
				LockType:        models.LockTypeShareUpdateExclusive,
				RowCount:        500000,
			},
			wantBlocksAll: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impact := calculator.CalculateImpact(tt.op)
			
			if impact == nil {
				t.Fatal("Expected lock impact, got nil")
			}

			if got := impact.BlocksReads; got != tt.wantBlocksAll {
				t.Errorf("BlocksReads = %v, want %v", got, tt.wantBlocksAll)
			}
		})
	}
}

func TestLockCalculator_EstimateDuration(t *testing.T) {
	calculator := concurrency.NewLockCalculator("postgresql")

	op := &models.Operation{
		Type:               models.OperationTypeAddColumn,
		RequiresRewrite:    true,
		RowCount:           1000000,
		EstimatedTimeSeconds: 30.0,
	}

	duration := calculator.EstimateLockDuration(op)

	if duration <= 0 {
		t.Error("Lock duration should be positive")
	}

	// Lock duration should be based on estimated time
	expectedMS := int64(30000) // 30 seconds
	tolerance := int64(5000)   // 5 second tolerance

	if duration < expectedMS-tolerance || duration > expectedMS+tolerance {
		t.Errorf("Lock duration %d ms not within expected range %d±%d ms", duration, expectedMS, tolerance)
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/analyzer/concurrency -v`
Expected: FAIL (undefined types)

### Step 3: Implement lock calculator

Create `internal/analyzer/concurrency/lock_calculator.go`:

```go
package concurrency

import (
	"github.com/iamsr/tapa/pkg/models"
)

// LockCalculator calculates lock impact for operations
type LockCalculator struct {
	databaseType string
}

// NewLockCalculator creates a new lock calculator
func NewLockCalculator(databaseType string) *LockCalculator {
	return &LockCalculator{
		databaseType: databaseType,
	}
}

// CalculateImpact calculates the lock impact for an operation
func (c *LockCalculator) CalculateImpact(op *models.Operation) *models.LockImpact {
	impact := &models.LockImpact{
		LockType:            op.LockType,
		EstimatedDurationMS: c.EstimateLockDuration(op),
	}

	// Determine what operations are blocked
	c.analyzeLockBehavior(impact, op)

	// Estimate blocked query count based on table access patterns
	impact.EstimatedBlockedCount = c.estimateBlockedQueries(op, impact)

	// Calculate wait time range
	impact.WaitTimeRange = c.calculateWaitTimeRange(impact.EstimatedDurationMS)

	// Assess lock acquisition risk
	impact.LockAcquisitionRisk = c.assessLockAcquisitionRisk(op, impact)

	return impact
}

// EstimateLockDuration estimates how long the lock will be held
func (c *LockCalculator) EstimateLockDuration(op *models.Operation) int64 {
	// For operations with estimated time, use that
	if op.EstimatedTimeSeconds > 0 {
		return int64(op.EstimatedTimeSeconds * 1000)
	}

	// For operations without estimated time, use heuristics
	baseMS := int64(100) // 100ms baseline

	// Increase based on row count
	if op.RowCount > 0 {
		// Rough estimate: 10ms per 10,000 rows
		baseMS += (op.RowCount / 10000) * 10
	}

	// Increase for rewrite operations
	if op.RequiresRewrite {
		baseMS *= 10
	}

	// Different operation types have different lock durations
	switch op.Type {
	case models.OperationTypeCreateIndex:
		// Indexes take longer
		baseMS *= 5
		if c.databaseType == "postgresql" && op.LockType == models.LockTypeShareUpdateExclusive {
			// CONCURRENTLY takes even longer
			baseMS *= 2
		}
	case models.OperationTypeDropColumn, models.OperationTypeDropTable:
		// Drops are usually fast
		baseMS = 50
	case models.OperationTypeAddColumn:
		if op.RequiresRewrite {
			// Adding column with default requires rewrite
			baseMS *= 20
		}
	}

	return baseMS
}

// analyzeLockBehavior determines what operations are blocked by the lock
func (c *LockCalculator) analyzeLockBehavior(impact *models.LockImpact, op *models.Operation) {
	switch impact.LockType {
	case models.LockTypeAccessExclusive:
		// Blocks everything
		impact.BlocksReads = true
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"SELECT", "INSERT", "UPDATE", "DELETE"}

	case models.LockTypeExclusive:
		// Blocks writes but not reads (in most cases)
		impact.BlocksReads = false
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"INSERT", "UPDATE", "DELETE"}

	case models.LockTypeShareUpdateExclusive:
		// Allows reads, blocks writes (concurrent index creation)
		impact.BlocksReads = false
		impact.BlocksWrites = false // Doesn't block INSERTs/UPDATEs for concurrent indexes
		impact.BlockedQueryTypes = []string{} // Minimal blocking

	case models.LockTypeShare:
		// Allows reads, blocks some writes
		impact.BlocksReads = false
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"UPDATE", "DELETE"}

	case models.LockTypeRowExclusive:
		// Row-level locking, minimal impact
		impact.BlocksReads = false
		impact.BlocksWrites = false
		impact.BlockedQueryTypes = []string{}

	default:
		// Conservative: assume blocks writes
		impact.BlocksReads = false
		impact.BlocksWrites = true
		impact.BlockedQueryTypes = []string{"INSERT", "UPDATE", "DELETE"}
	}
}

// estimateBlockedQueries estimates how many queries will be blocked
func (c *LockCalculator) estimateBlockedQueries(op *models.Operation, impact *models.LockImpact) int {
	if !impact.BlocksReads && !impact.BlocksWrites {
		return 0
	}

	// Base estimate on lock duration and typical query rate
	// Assume average database handles ~100 queries/second to this table
	queriesPerSecond := 10.0 // Conservative estimate for single table
	
	// Scale based on table size (larger tables likely have more traffic)
	if op.RowCount > 1000000 {
		queriesPerSecond = 50.0 // Larger tables get more queries
	} else if op.RowCount > 100000 {
		queriesPerSecond = 25.0
	}

	// Calculate blocked queries
	lockDurationSeconds := float64(impact.EstimatedDurationMS) / 1000.0
	estimatedBlocked := int(queriesPerSecond * lockDurationSeconds)

	// Adjust based on what's blocked
	if impact.BlocksReads && impact.BlocksWrites {
		// Blocks everything
		return estimatedBlocked
	} else if impact.BlocksWrites {
		// Only blocks writes (typically ~30% of queries)
		return estimatedBlocked / 3
	} else {
		// Minimal blocking
		return estimatedBlocked / 10
	}
}

// calculateWaitTimeRange converts duration to human-readable range
func (c *LockCalculator) calculateWaitTimeRange(durationMS int64) string {
	seconds := durationMS / 1000

	switch {
	case seconds < 1:
		return "<1 second"
	case seconds < 2:
		return "1-2 seconds"
	case seconds < 5:
		return "2-5 seconds"
	case seconds < 10:
		return "5-10 seconds"
	case seconds < 30:
		return "10-30 seconds"
	case seconds < 60:
		return "30-60 seconds"
	case seconds < 300:
		return "1-5 minutes"
	default:
		return ">5 minutes"
	}
}

// assessLockAcquisitionRisk assesses how risky it is to acquire the lock
func (c *LockCalculator) assessLockAcquisitionRisk(op *models.Operation, impact *models.LockImpact) string {
	// ACCESS EXCLUSIVE locks are always high risk
	if impact.LockType == models.LockTypeAccessExclusive {
		if impact.EstimatedDurationMS > 30000 {
			return "critical" // >30s ACCESS EXCLUSIVE is critical
		}
		return "high"
	}

	// Long duration locks are higher risk
	if impact.EstimatedDurationMS > 60000 {
		return "high"
	} else if impact.EstimatedDurationMS > 10000 {
		return "medium"
	}

	// Minimal impact locks are low risk
	if !impact.BlocksReads && !impact.BlocksWrites {
		return "low"
	}

	return "medium"
}
```

### Step 4: Run test to verify it passes

Run: `go test ./internal/analyzer/concurrency -v`
Expected: PASS

### Step 5: Commit

```bash
git add internal/analyzer/concurrency/lock_calculator.go internal/analyzer/concurrency/lock_calculator_test.go
git commit -m "feat(concurrency): implement lock impact calculator"
```

---

## Task 3: Workload Analyzer

**Goal:** Analyze database workload patterns to predict concurrency impact

**Files:**
- Create: `internal/analyzer/concurrency/workload_analyzer.go`
- Create: `internal/analyzer/concurrency/workload_analyzer_test.go`

### Step 1: Write failing test for workload analyzer

Create test file:

```go
package concurrency_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/pkg/models"
)

func TestWorkloadAnalyzer_AnalyzeWorkload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	analyzer := concurrency.NewWorkloadAnalyzer("postgresql", nil)

	ctx := context.Background()
	workload, err := analyzer.AnalyzeWorkload(ctx, "users")

	// In mock mode (nil DB), should return defaults or skip
	if err != nil {
		t.Fatalf("AnalyzeWorkload failed: %v", err)
	}

	if workload != nil {
		if workload.ActiveConnections < 0 {
			t.Error("ActiveConnections should not be negative")
		}

		if workload.QueriesPerSecond < 0 {
			t.Error("QueriesPerSecond should not be negative")
		}
	}
}

func TestWorkloadAnalyzer_ClassifyAccessFrequency(t *testing.T) {
	analyzer := concurrency.NewWorkloadAnalyzer("postgresql", nil)

	tests := []struct {
		name           string
		queriesPerMin  int
		expectedLevel  string
	}{
		{"low traffic", 5, "low"},
		{"medium traffic", 150, "medium"},
		{"high traffic", 800, "high"},
		{"very high traffic", 2000, "very_high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frequency := analyzer.ClassifyAccessFrequency(tt.queriesPerMin)
			if frequency != tt.expectedLevel {
				t.Errorf("ClassifyAccessFrequency(%d) = %s, want %s", tt.queriesPerMin, frequency, tt.expectedLevel)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/analyzer/concurrency -v -run TestWorkload`
Expected: FAIL (undefined types)

### Step 3: Implement workload analyzer

Create `internal/analyzer/concurrency/workload_analyzer.go`:

```go
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

// AnalyzeWorkload analyzes current workload for a table
func (w *WorkloadAnalyzer) AnalyzeWorkload(ctx context.Context, tableName string) (*models.WorkloadAnalysis, error) {
	// Mock mode: return default workload
	if w.db == nil {
		return w.mockWorkload(), nil
	}

	switch w.databaseType {
	case "postgresql":
		return w.analyzePostgreSQLWorkload(ctx, tableName)
	case "mysql":
		return w.analyzeMySQLWorkload(ctx, tableName)
	default:
		return w.mockWorkload(), nil
	}
}

// analyzePostgreSQLWorkload queries PostgreSQL system tables
func (w *WorkloadAnalyzer) analyzePostgreSQLWorkload(ctx context.Context, tableName string) (*models.WorkloadAnalysis, error) {
	workload := &models.WorkloadAnalysis{}

	// Get active connections
	var activeConns int
	err := w.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&activeConns)
	if err != nil {
		// Non-fatal: use defaults
		activeConns = 0
	}
	workload.ActiveConnections = activeConns

	// Get table statistics (requires pg_stat_statements extension)
	// This is optional - if extension not available, skip
	var queriesPerMin float64
	query := `
		SELECT 
			COALESCE(SUM(calls), 0) / 
			GREATEST(EXTRACT(EPOCH FROM (NOW() - stats_reset)) / 60, 1)
		FROM pg_stat_statements pss
		JOIN pg_class c ON pss.queryid::text LIKE '%' || c.relname || '%'
		WHERE c.relname = $1
	`
	err = w.db.QueryRowContext(ctx, query, tableName).Scan(&queriesPerMin)
	if err != nil {
		// Extension not available or other error - use estimate
		queriesPerMin = 10.0 // Conservative default
	}
	workload.QueriesPerSecond = queriesPerMin / 60.0

	// Classify access frequency
	workload.TableAccessFrequency = w.ClassifyAccessFrequency(int(queriesPerMin))

	// Check for long-running queries
	var longRunning int
	err = w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE state = 'active' 
		AND NOW() - query_start > INTERVAL '5 seconds'
	`).Scan(&longRunning)
	if err != nil {
		longRunning = 0
	}
	workload.LongRunningQueries = longRunning

	// Determine if peak load period (heuristic based on connections)
	workload.PeakLoadPeriod = activeConns > 50

	return workload, nil
}

// analyzeMySQLWorkload queries MySQL system tables
func (w *WorkloadAnalyzer) analyzeMySQLWorkload(ctx context.Context, tableName string) (*models.WorkloadAnalysis, error) {
	workload := &models.WorkloadAnalysis{}

	// Get active connections
	var activeConns int
	err := w.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.processlist WHERE command != 'Sleep'").Scan(&activeConns)
	if err != nil {
		activeConns = 0
	}
	workload.ActiveConnections = activeConns

	// MySQL doesn't have easy per-table query stats without performance_schema
	// Use conservative estimates
	workload.QueriesPerSecond = 10.0
	workload.TableAccessFrequency = "medium"
	workload.PeakLoadPeriod = activeConns > 50

	// Check for long-running queries
	var longRunning int
	err = w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM information_schema.processlist 
		WHERE command != 'Sleep' 
		AND time > 5
	`).Scan(&longRunning)
	if err != nil {
		longRunning = 0
	}
	workload.LongRunningQueries = longRunning

	return workload, nil
}

// mockWorkload returns default workload for testing
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

// ClassifyAccessFrequency classifies table access frequency
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
```

### Step 4: Run test to verify it passes

Run: `go test ./internal/analyzer/concurrency -v -run TestWorkload`
Expected: PASS

### Step 5: Commit

```bash
git add internal/analyzer/concurrency/workload_analyzer.go internal/analyzer/concurrency/workload_analyzer_test.go
git commit -m "feat(concurrency): implement workload analyzer"
```

---

## Task 4: Concurrency Analyzer

**Goal:** Main analyzer that orchestrates concurrency analysis

**Files:**
- Create: `internal/analyzer/concurrency/analyzer.go`
- Create: `internal/analyzer/concurrency/analyzer_test.go`
- Modify: `pkg/models/operation.go` (add ConcurrencyAnalysis field)

### Step 1: Write failing test for concurrency analyzer

Create test file:

```go
package concurrency_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/analyzer/concurrency"
	"github.com/iamsr/tapa/pkg/models"
)

func TestAnalyzer_AnalyzeOperation(t *testing.T) {
	analyzer := concurrency.NewAnalyzer("postgresql", nil)

	tests := []struct {
		name              string
		op                *models.Operation
		expectHighImpact  bool
	}{
		{
			name: "ACCESS EXCLUSIVE on large table",
			op: &models.Operation{
				Type:               models.OperationTypeAddColumn,
				TableName:          "users",
				RequiresRewrite:    true,
				LockType:           models.LockTypeAccessExclusive,
				RowCount:           5000000,
				EstimatedTimeSeconds: 120.0,
			},
			expectHighImpact: true,
		},
		{
			name: "CREATE INDEX CONCURRENTLY low impact",
			op: &models.Operation{
				Type:            models.OperationTypeCreateIndex,
				TableName:       "orders",
				RequiresRewrite: false,
				LockType:        models.LockTypeShareUpdateExclusive,
				RowCount:        1000000,
			},
			expectHighImpact: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzer.AnalyzeOperation(ctx, tt.op)
			if err != nil {
				t.Fatalf("AnalyzeOperation failed: %v", err)
			}

			if tt.op.ConcurrencyAnalysis == nil {
				t.Fatal("ConcurrencyAnalysis should be populated")
			}

			if got := tt.op.ConcurrencyAnalysis.IsHighImpact(); got != tt.expectHighImpact {
				t.Errorf("IsHighImpact() = %v, want %v (score: %d)", got, tt.expectHighImpact, tt.op.ConcurrencyAnalysis.ImpactScore)
			}
		})
	}
}

func TestAnalyzer_GenerateAlternatives(t *testing.T) {
	analyzer := concurrency.NewAnalyzer("postgresql", nil)

	op := &models.Operation{
		Type:               models.OperationTypeCreateIndex,
		TableName:          "users",
		IndexName:          "idx_users_email",
		LockType:           models.LockTypeShare,
		RowCount:           1000000,
		EstimatedTimeSeconds: 60.0,
	}

	alternatives := analyzer.GenerateAlternatives(op)

	// Should suggest CONCURRENTLY for PostgreSQL
	if len(alternatives) == 0 {
		t.Error("Expected at least one alternative")
	}

	foundConcurrent := false
	for _, alt := range alternatives {
		if alt.LockType == models.LockTypeShareUpdateExclusive {
			foundConcurrent = true
			break
		}
	}

	if !foundConcurrent {
		t.Error("Expected CONCURRENT alternative for CREATE INDEX")
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/analyzer/concurrency -v -run TestAnalyzer`
Expected: FAIL (undefined types)

### Step 3: Add ConcurrencyAnalysis field to Operation

Modify `pkg/models/operation.go`:

```go
type Operation struct {
	// ... existing fields ...
	DataMigrationAnalysis *DataMigrationAnalysis `json:"data_migration_analysis,omitempty"`
	ConcurrencyAnalysis   *ConcurrencyAnalysis   `json:"concurrency_analysis,omitempty"` // NEW
}
```

### Step 4: Implement concurrency analyzer

Create `internal/analyzer/concurrency/analyzer.go`:

```go
package concurrency

import (
	"context"
	"database/sql"

	"github.com/iamsr/tapa/pkg/models"
)

// Analyzer performs concurrency impact analysis
type Analyzer struct {
	databaseType      string
	db                *sql.DB
	lockCalculator    *LockCalculator
	workloadAnalyzer  *WorkloadAnalyzer
}

// NewAnalyzer creates a new concurrency analyzer
func NewAnalyzer(databaseType string, db *sql.DB) *Analyzer {
	return &Analyzer{
		databaseType:     databaseType,
		db:               db,
		lockCalculator:   NewLockCalculator(databaseType),
		workloadAnalyzer: NewWorkloadAnalyzer(databaseType, db),
	}
}

// AnalyzeOperation performs concurrency analysis for a single operation
func (a *Analyzer) AnalyzeOperation(ctx context.Context, op *models.Operation) error {
	analysis := &models.ConcurrencyAnalysis{
		ConcurrencySafe: true,
	}

	// Calculate lock impact
	lockImpact := a.lockCalculator.CalculateImpact(op)
	analysis.LockImpact = lockImpact

	// Analyze current workload (optional, requires DB connection)
	if a.db != nil {
		workload, err := a.workloadAnalyzer.AnalyzeWorkload(ctx, op.TableName)
		if err == nil {
			analysis.WorkloadAnalysis = workload
			
			// Update blocked query count based on actual workload
			if workload != nil {
				lockImpact.EstimatedBlockedCount = a.workloadAnalyzer.EstimateBlockedQueries(
					workload,
					lockImpact.EstimatedDurationMS,
					lockImpact.BlockedQueryTypes,
				)
			}
		}
	}

	// Calculate impact score
	analysis.ImpactScore = a.calculateImpactScore(op, lockImpact, analysis.WorkloadAnalysis)

	// Determine if concurrency safe
	analysis.ConcurrencySafe = !lockImpact.BlocksReads && !lockImpact.BlocksWrites

	// Generate safer alternatives
	analysis.SaferAlternatives = a.GenerateAlternatives(op)

	// Generate recommendations
	analysis.Recommendations = a.generateRecommendations(op, lockImpact, analysis.WorkloadAnalysis)

	// Estimate downtime
	if lockImpact.BlocksReads || lockImpact.BlocksWrites {
		analysis.EstimatedDowntimeMS = lockImpact.EstimatedDurationMS
	}

	// Attach to operation
	op.ConcurrencyAnalysis = analysis

	return nil
}

// calculateImpactScore calculates concurrency impact score (0-100)
func (a *Analyzer) calculateImpactScore(op *models.Operation, lockImpact *models.LockImpact, workload *models.WorkloadAnalysis) int {
	score := 0

	// Lock type factor (0-40 points)
	switch lockImpact.LockType {
	case models.LockTypeAccessExclusive:
		score += 40 // Most restrictive
	case models.LockTypeExclusive:
		score += 30
	case models.LockTypeShare:
		score += 20
	case models.LockTypeShareUpdateExclusive:
		score += 10 // Concurrent operations
	case models.LockTypeRowExclusive:
		score += 5
	default:
		score += 15
	}

	// Lock duration factor (0-30 points)
	durationSeconds := lockImpact.EstimatedDurationMS / 1000
	switch {
	case durationSeconds > 300: // >5 minutes
		score += 30
	case durationSeconds > 60: // >1 minute
		score += 25
	case durationSeconds > 30: // >30 seconds
		score += 20
	case durationSeconds > 10: // >10 seconds
		score += 15
	case durationSeconds > 5: // >5 seconds
		score += 10
	default:
		score += 5
	}

	// Workload factor (0-20 points)
	if workload != nil {
		switch workload.TableAccessFrequency {
		case "very_high":
			score += 20
		case "high":
			score += 15
		case "medium":
			score += 10
		case "low":
			score += 5
		}

		// Add points for peak load
		if workload.PeakLoadPeriod {
			score += 5
		}
	} else {
		// Conservative default
		score += 10
	}

	// Blocked operations factor (0-10 points)
	if lockImpact.BlocksReads && lockImpact.BlocksWrites {
		score += 10
	} else if lockImpact.BlocksWrites {
		score += 5
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// GenerateAlternatives generates safer concurrency alternatives
func (a *Analyzer) GenerateAlternatives(op *models.Operation) []models.ConcurrentAlternative {
	var alternatives []models.ConcurrentAlternative

	switch op.Type {
	case models.OperationTypeCreateIndex:
		if a.databaseType == "postgresql" && op.LockType != models.LockTypeShareUpdateExclusive {
			alternatives = append(alternatives, models.ConcurrentAlternative{
				Description:     "Use CREATE INDEX CONCURRENTLY to reduce lock impact",
				LockType:        models.LockTypeShareUpdateExclusive,
				ImpactReduction: 70,
				RequiresFeature: "PostgreSQL 8.2+",
				Steps: []string{
					"Replace CREATE INDEX with CREATE INDEX CONCURRENTLY",
					"Monitor for longer execution time",
					"Ensure no other DDL running concurrently",
				},
				EstimatedTimeMS: int64(op.EstimatedTimeSeconds * 1500), // 50% longer
				Tradeoffs: []string{
					"Takes 50-100% longer to complete",
					"Cannot be run in transaction block",
					"May fail and leave invalid index",
				},
			})
		}

	case models.OperationTypeAddColumn:
		if op.RequiresRewrite {
			alternatives = append(alternatives, models.ConcurrentAlternative{
				Description:     "Add column without default, then backfill in batches",
				LockType:        models.LockTypeAccessExclusive,
				ImpactReduction: 80,
				RequiresFeature: "Manual multi-step process",
				Steps: []string{
					"Step 1: ALTER TABLE ADD COLUMN (no default) - fast, brief lock",
					"Step 2: UPDATE table SET column = value in batches - no table lock",
					"Step 3: ALTER TABLE ALTER COLUMN SET DEFAULT - fast, brief lock",
				},
				EstimatedTimeMS: int64(op.EstimatedTimeSeconds * 1200), // 20% longer total
				Tradeoffs: []string{
					"Requires manual batching logic",
					"Column temporarily NULL during backfill",
					"More complex deployment process",
				},
			})
		}

	case models.OperationTypeAlterColumn:
		if a.databaseType == "postgresql" {
			alternatives = append(alternatives, models.ConcurrentAlternative{
				Description:     "Create new column, migrate data, swap columns",
				LockType:        models.LockTypeAccessExclusive,
				ImpactReduction: 75,
				RequiresFeature: "Multi-step migration process",
				Steps: []string{
					"Step 1: Add new column with desired type",
					"Step 2: Backfill data in batches",
					"Step 3: Swap column names (brief lock)",
					"Step 4: Drop old column",
				},
				EstimatedTimeMS: int64(op.EstimatedTimeSeconds * 1500),
				Tradeoffs: []string{
					"Requires 2x storage during migration",
					"Complex multi-step process",
					"Risk of data inconsistency during transition",
				},
			})
		}

	case models.OperationTypeDropColumn:
		alternatives = append(alternatives, models.ConcurrentAlternative{
			Description:     "Two-phase drop: ignore in app, then drop column later",
			LockType:        models.LockTypeAccessExclusive,
			ImpactReduction: 90,
			RequiresFeature: "Application change coordination",
			Steps: []string{
				"Phase 1: Deploy app that no longer references column",
				"Phase 2: Wait for all old app instances to stop",
				"Phase 3: DROP COLUMN during maintenance window",
			},
			EstimatedTimeMS: 1000, // Actual drop is fast
			Tradeoffs: []string{
				"Requires coordinated deployment",
				"Column remains in database until Phase 3",
				"Takes longer calendar time",
			},
		})
	}

	return alternatives
}

// generateRecommendations generates concurrency-specific recommendations
func (a *Analyzer) generateRecommendations(op *models.Operation, lockImpact *models.LockImpact, workload *models.WorkloadAnalysis) []string {
	var recommendations []string

	// High impact operations
	if lockImpact.EstimatedDurationMS > 30000 && lockImpact.BlocksReads {
		recommendations = append(recommendations, "⚠️  HIGH IMPACT: This operation will block all queries for >30 seconds")
		recommendations = append(recommendations, "Consider scheduling during maintenance window")
	}

	// Workload-based recommendations
	if workload != nil {
		if workload.PeakLoadPeriod {
			recommendations = append(recommendations, "⚠️  Current workload is HIGH - consider delaying migration")
		}

		if workload.LongRunningQueries > 5 {
			recommendations = append(recommendations, fmt.Sprintf("⚠️  %d long-running queries detected - may delay lock acquisition", workload.LongRunningQueries))
		}
	}

	// Lock acquisition recommendations
	if lockImpact.LockAcquisitionRisk == "high" || lockImpact.LockAcquisitionRisk == "critical" {
		recommendations = append(recommendations, "Set statement_timeout to prevent indefinite blocking")
		recommendations = append(recommendations, "Consider using lock_timeout to fail fast if lock unavailable")
	}

	// Alternatives available
	if len(a.GenerateAlternatives(op)) > 0 {
		recommendations = append(recommendations, "✓ Safer concurrency alternatives available (see alternatives)")
	}

	// General best practices
	if lockImpact.EstimatedDurationMS > 5000 {
		recommendations = append(recommendations, "Monitor active queries before executing: SELECT * FROM pg_stat_activity")
		recommendations = append(recommendations, "Prepare rollback plan in case of issues")
	}

	return recommendations
}

// AnalyzeMigration performs concurrency analysis for all operations
func (a *Analyzer) AnalyzeMigration(ctx context.Context, migration *models.Migration) error {
	for _, op := range migration.Operations {
		if err := a.AnalyzeOperation(ctx, op); err != nil {
			return err
		}
	}
	return nil
}
```

### Step 5: Run test to verify it passes

Run: `go test ./internal/analyzer/concurrency -v -run TestAnalyzer`
Expected: PASS

### Step 6: Commit

```bash
git add internal/analyzer/concurrency/analyzer.go internal/analyzer/concurrency/analyzer_test.go pkg/models/operation.go
git commit -m "feat(concurrency): implement concurrency analyzer"
```

---

## Task 5: CLI Integration

**Goal:** Add `--concurrency` flag to CLI and integrate with comprehensive mode

**Files:**
- Modify: `cmd/tapa/analyze.go`
- Modify: `internal/analyzer/postgres/analyzer.go`

### Step 1: Add CLI flag

Modify `cmd/tapa/analyze.go`:

```go
func init() {
	// Existing flags...
	analyzeCmd.Flags().Bool("concurrency", false, "Analyze concurrency impact of migration")
	// Note: --comprehensive already includes concurrency analysis
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// ... existing code ...

	comprehensive, _ := cmd.Flags().GetBool("comprehensive")
	concurrency, _ := cmd.Flags().GetBool("concurrency")

	// Concurrency enabled if flag set or comprehensive mode
	enableConcurrency := concurrency || comprehensive

	var concurrencyAnalyzer *concurrency.Analyzer
	if enableConcurrency && dbURL != "" {
		// Connect to database
		db, err := sql.Open(dbType, dbURL)
		if err != nil {
			// Non-fatal: continue without concurrency analysis
			fmt.Fprintf(os.Stderr, "Warning: Failed to connect for concurrency analysis: %v\n", err)
		} else {
			defer db.Close()
			concurrencyAnalyzer = concurrency.NewAnalyzer(dbType, db)
		}
	}

	// ... analyze each operation ...

	if concurrencyAnalyzer != nil {
		if err := concurrencyAnalyzer.AnalyzeOperation(ctx, op); err != nil {
			// Non-fatal: continue
			fmt.Fprintf(os.Stderr, "Warning: Concurrency analysis failed: %v\n", err)
		}
	}

	// ... rest of code ...
}
```

### Step 2: Update PostgreSQL analyzer

Modify `internal/analyzer/postgres/analyzer.go`:

```go
// Add to Analyze function
if comprehensive {
	// Existing analyzers...
	
	// NEW: Concurrency analyzer
	if concurrencyAnalyzer != nil {
		if err := concurrencyAnalyzer.AnalyzeMigration(ctx, migration); err != nil {
			// Non-fatal
		}
	}
}
```

### Step 3: Test CLI flag

Run: `go run cmd/tapa/main.go analyze --help | grep concurrency`
Expected: Flag appears in help text

### Step 4: Commit

```bash
git add cmd/tapa/analyze.go internal/analyzer/postgres/analyzer.go
git commit -m "feat(concurrency): add --concurrency CLI flag"
```

---

## Task 6: Output Formatter

**Goal:** Display concurrency analysis results in human-readable format

**Files:**
- Create: `internal/output/concurrency_formatter.go`
- Create: `internal/output/concurrency_formatter_test.go`
- Modify: `internal/output/advanced_formatter.go`

### Step 1: Write test for formatter

Create test file:

```go
package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iamsr/tapa/internal/output"
	"github.com/iamsr/tapa/pkg/models"
)

func TestFormatConcurrencyAnalysis(t *testing.T) {
	analysis := &models.ConcurrencyAnalysis{
		ImpactScore: 75,
		LockImpact: &models.LockImpact{
			LockType:              models.LockTypeAccessExclusive,
			EstimatedDurationMS:   45000,
			BlocksReads:           true,
			BlocksWrites:          true,
			BlockedQueryTypes:     []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
			EstimatedBlockedCount: 150,
			WaitTimeRange:         "30-60 seconds",
			LockAcquisitionRisk:   "high",
		},
		WorkloadAnalysis: &models.WorkloadAnalysis{
			ActiveConnections:    25,
			QueriesPerSecond:     50.5,
			TableAccessFrequency: "high",
			PeakLoadPeriod:       true,
		},
		SaferAlternatives: []models.ConcurrentAlternative{
			{
				Description:     "Use CREATE INDEX CONCURRENTLY",
				LockType:        models.LockTypeShareUpdateExclusive,
				ImpactReduction: 70,
			},
		},
		Recommendations: []string{
			"Schedule during maintenance window",
			"Monitor active queries before executing",
		},
		ConcurrencySafe: false,
	}

	var buf bytes.Buffer
	err := output.FormatConcurrencyAnalysis(&buf, analysis)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis failed: %v", err)
	}

	outputStr := buf.String()

	// Verify key sections are present
	if !strings.Contains(outputStr, "Concurrency Impact") {
		t.Error("Output should contain 'Concurrency Impact' section")
	}

	if !strings.Contains(outputStr, "HIGH") {
		t.Error("Output should show HIGH impact level")
	}

	if !strings.Contains(outputStr, "75") {
		t.Error("Output should show impact score")
	}

	if !strings.Contains(outputStr, "ACCESS_EXCLUSIVE") {
		t.Error("Output should show lock type")
	}

	if !strings.Contains(outputStr, "150 queries") {
		t.Error("Output should show blocked query count")
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/output -v -run TestFormatConcurrency`
Expected: FAIL (undefined function)

### Step 3: Implement formatter

Create `internal/output/concurrency_formatter.go`:

```go
package output

import (
	"fmt"
	"io"

	"github.com/iamsr/tapa/pkg/models"
)

// FormatConcurrencyAnalysis formats concurrency analysis results
func FormatConcurrencyAnalysis(w io.Writer, analysis *models.ConcurrencyAnalysis) error {
	if analysis == nil {
		return nil
	}

	fmt.Fprintln(w, "\nConcurrency Impact:")
	fmt.Fprintln(w, "─────────────────────────────────────")

	// Impact level with color
	levelColor := getImpactLevelColor(analysis.ImpactLevel())
	fmt.Fprintf(w, "Impact Level: %s%s%s (score: %d/100)\n", 
		levelColor, analysis.ImpactLevel(), colorReset, analysis.ImpactScore)

	// Concurrency safe indicator
	if analysis.ConcurrencySafe {
		fmt.Fprintf(w, "Concurrency Safe: %s✓ YES%s\n", colorGreen, colorReset)
	} else {
		fmt.Fprintf(w, "Concurrency Safe: %s✗ NO%s\n", colorRed, colorReset)
	}

	// Lock impact details
	if analysis.LockImpact != nil {
		fmt.Fprintln(w, "\nLock Details:")
		fmt.Fprintf(w, "  Lock Type: %s\n", analysis.LockImpact.LockType)
		fmt.Fprintf(w, "  Duration: %s (%d ms)\n", 
			analysis.LockImpact.WaitTimeRange, analysis.LockImpact.EstimatedDurationMS)
		
		// What gets blocked
		if analysis.LockImpact.BlocksReads && analysis.LockImpact.BlocksWrites {
			fmt.Fprintf(w, "  %sBlocks: ALL operations (reads + writes)%s\n", colorRed, colorReset)
		} else if analysis.LockImpact.BlocksWrites {
			fmt.Fprintf(w, "  %sBlocks: Write operations only%s\n", colorYellow, colorReset)
		} else {
			fmt.Fprintf(w, "  %sBlocks: Minimal (concurrent-safe)%s\n", colorGreen, colorReset)
		}

		// Blocked query count
		if analysis.LockImpact.EstimatedBlockedCount > 0 {
			fmt.Fprintf(w, "  Estimated blocked queries: %s%d%s\n", 
				colorRed, analysis.LockImpact.EstimatedBlockedCount, colorReset)
		}

		// Lock acquisition risk
		riskColor := colorGreen
		if analysis.LockImpact.LockAcquisitionRisk == "high" || analysis.LockImpact.LockAcquisitionRisk == "critical" {
			riskColor = colorRed
		} else if analysis.LockImpact.LockAcquisitionRisk == "medium" {
			riskColor = colorYellow
		}
		fmt.Fprintf(w, "  Lock acquisition risk: %s%s%s\n", 
			riskColor, analysis.LockImpact.LockAcquisitionRisk, colorReset)
	}

	// Workload analysis
	if analysis.WorkloadAnalysis != nil {
		fmt.Fprintln(w, "\nCurrent Workload:")
		fmt.Fprintf(w, "  Active connections: %d\n", analysis.WorkloadAnalysis.ActiveConnections)
		fmt.Fprintf(w, "  Queries/second: %.1f\n", analysis.WorkloadAnalysis.QueriesPerSecond)
		fmt.Fprintf(w, "  Table access: %s\n", analysis.WorkloadAnalysis.TableAccessFrequency)
		
		if analysis.WorkloadAnalysis.PeakLoadPeriod {
			fmt.Fprintf(w, "  %s⚠️  PEAK LOAD PERIOD%s\n", colorRed, colorReset)
		}

		if analysis.WorkloadAnalysis.LongRunningQueries > 0 {
			fmt.Fprintf(w, "  Long-running queries: %s%d%s (may delay lock)\n",
				colorYellow, analysis.WorkloadAnalysis.LongRunningQueries, colorReset)
		}
	}

	// Estimated downtime
	if analysis.EstimatedDowntimeMS > 0 {
		downtimeSeconds := analysis.EstimatedDowntimeMS / 1000
		downtimeColor := colorGreen
		if downtimeSeconds > 60 {
			downtimeColor = colorRed
		} else if downtimeSeconds > 10 {
			downtimeColor = colorYellow
		}
		fmt.Fprintf(w, "\nEstimated Downtime: %s%d seconds%s\n", 
			downtimeColor, downtimeSeconds, colorReset)
	}

	// Safer alternatives
	if len(analysis.SaferAlternatives) > 0 {
		fmt.Fprintln(w, "\nSafer Alternatives:")
		for i, alt := range analysis.SaferAlternatives {
			fmt.Fprintf(w, "\n  %d. %s\n", i+1, alt.Description)
			fmt.Fprintf(w, "     Lock type: %s\n", alt.LockType)
			fmt.Fprintf(w, "     Impact reduction: %s-%d%%%s\n", colorGreen, alt.ImpactReduction, colorReset)
			
			if alt.RequiresFeature != "" {
				fmt.Fprintf(w, "     Requires: %s\n", alt.RequiresFeature)
			}

			if len(alt.Steps) > 0 {
				fmt.Fprintln(w, "     Steps:")
				for _, step := range alt.Steps {
					fmt.Fprintf(w, "       - %s\n", step)
				}
			}

			if len(alt.Tradeoffs) > 0 {
				fmt.Fprintln(w, "     Tradeoffs:")
				for _, tradeoff := range alt.Tradeoffs {
					fmt.Fprintf(w, "       • %s\n", tradeoff)
				}
			}
		}
	}

	// Recommendations
	if len(analysis.Recommendations) > 0 {
		fmt.Fprintln(w, "\nRecommendations:")
		for _, rec := range analysis.Recommendations {
			fmt.Fprintf(w, "  • %s\n", rec)
		}
	}

	return nil
}

// getImpactLevelColor returns ANSI color for impact level
func getImpactLevelColor(level models.ConcurrencyImpactLevel) string {
	switch level {
	case models.ConcurrencyImpactMinimal, models.ConcurrencyImpactLow:
		return colorGreen
	case models.ConcurrencyImpactMedium:
		return colorYellow
	case models.ConcurrencyImpactHigh, models.ConcurrencyImpactCritical:
		return colorRed
	default:
		return colorReset
	}
}
```

### Step 4: Integrate with advanced formatter

Modify `internal/output/advanced_formatter.go`:

```go
// Add to FormatAdvancedFeatures function
func FormatAdvancedFeatures(w io.Writer, op *models.Operation) error {
	// ... existing formatters ...

	// NEW: Concurrency analysis
	if op.ConcurrencyAnalysis != nil {
		if err := FormatConcurrencyAnalysis(w, op.ConcurrencyAnalysis); err != nil {
			return err
		}
	}

	return nil
}
```

### Step 5: Run test to verify it passes

Run: `go test ./internal/output -v -run TestFormatConcurrency`
Expected: PASS

### Step 6: Commit

```bash
git add internal/output/concurrency_formatter.go internal/output/concurrency_formatter_test.go internal/output/advanced_formatter.go
git commit -m "feat(concurrency): add output formatter for concurrency analysis"
```

---

## Task 7: Integration Test

**Goal:** End-to-end test verifying concurrency analysis

**Files:**
- Modify: `tests/integration/advanced_features_test.go`

### Step 1: Add concurrency tests to integration suite

Modify existing integration test file:

```go
func TestConcurrencyAnalysis_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	migrationSQL := `
	-- High impact: ACCESS EXCLUSIVE lock on large table
	ALTER TABLE users ADD COLUMN last_login TIMESTAMP DEFAULT NOW();

	-- Low impact: Concurrent index creation
	CREATE INDEX CONCURRENTLY idx_users_email ON users(email);

	-- Medium impact: Regular index creation
	CREATE INDEX idx_orders_user_id ON orders(user_id);
	`

	pgParser := parser.GetParser("postgresql")
	migration, err := pgParser.Parse(migrationSQL)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Create concurrency analyzer (nil DB for mock mode)
	analyzer := concurrency.NewAnalyzer("postgresql", nil)

	// Analyze all operations
	ctx := context.Background()
	err = analyzer.AnalyzeMigration(ctx, migration)
	if err != nil {
		t.Fatalf("AnalyzeMigration failed: %v", err)
	}

	// Test high impact operation
	t.Run("HighImpactOperation", func(t *testing.T) {
		op := migration.Operations[0]
		if op.ConcurrencyAnalysis == nil {
			t.Fatal("ConcurrencyAnalysis should be populated")
		}

		if !op.ConcurrencyAnalysis.IsHighImpact() {
			t.Errorf("Expected high impact for ADD COLUMN with DEFAULT, got score %d", 
				op.ConcurrencyAnalysis.ImpactScore)
		}

		if op.ConcurrencyAnalysis.ConcurrencySafe {
			t.Error("ADD COLUMN with DEFAULT should not be concurrency safe")
		}

		if len(op.ConcurrencyAnalysis.SaferAlternatives) == 0 {
			t.Error("Expected safer alternatives for high-impact operation")
		}
	})

	// Test concurrent index creation
	t.Run("ConcurrentIndexLowImpact", func(t *testing.T) {
		if len(migration.Operations) < 2 {
			t.Skip("Not enough operations parsed")
		}

		op := migration.Operations[1]
		if op.ConcurrencyAnalysis == nil {
			t.Fatal("ConcurrencyAnalysis should be populated")
		}

		if op.ConcurrencyAnalysis.IsHighImpact() {
			t.Errorf("CONCURRENT index should have low impact, got score %d", 
				op.ConcurrencyAnalysis.ImpactScore)
		}
	})

	// Test alternatives generation
	t.Run("AlternativesGenerated", func(t *testing.T) {
		if len(migration.Operations) < 3 {
			t.Skip("Not enough operations parsed")
		}

		op := migration.Operations[2]
		if op.ConcurrencyAnalysis == nil {
			t.Fatal("ConcurrencyAnalysis should be populated")
		}

		// Regular CREATE INDEX should have CONCURRENT alternative
		foundConcurrentAlt := false
		for _, alt := range op.ConcurrencyAnalysis.SaferAlternatives {
			if alt.LockType == models.LockTypeShareUpdateExclusive {
				foundConcurrentAlt = true
				break
			}
		}

		if !foundConcurrentAlt {
			t.Error("Expected CONCURRENT alternative for CREATE INDEX")
		}
	})
}

func TestConcurrencyAnalysis_ComprehensiveMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that comprehensive mode includes concurrency analysis
	migrationSQL := `ALTER TABLE products ADD COLUMN price DECIMAL(10,2) NOT NULL DEFAULT 0.00;`

	pgParser := parser.GetParser("postgresql")
	migration, err := pgParser.Parse(migrationSQL)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Simulate comprehensive mode (all analyzers enabled)
	ctx := context.Background()

	diskAnalyzer := diskspace.NewAnalyzer("postgresql", nil)
	rollbackAnalyzer := rollback.NewAnalyzer("postgresql")
	dataMigrationAnalyzer := datamigration.NewAnalyzer("postgresql")
	concurrencyAnalyzer := concurrency.NewAnalyzer("postgresql", nil)

	for _, op := range migration.Operations {
		_ = diskAnalyzer.AnalyzeOperation(ctx, op)
		_ = rollbackAnalyzer.AnalyzeOperation(op)
		_ = dataMigrationAnalyzer.AnalyzeOperation(ctx, op)
		_ = concurrencyAnalyzer.AnalyzeOperation(ctx, op)
	}

	op := migration.Operations[0]

	// Verify all analyses are present
	if op.DiskSpaceAnalysis == nil {
		t.Error("DiskSpaceAnalysis should be populated in comprehensive mode")
	}

	if op.RollbackAnalysis == nil {
		t.Error("RollbackAnalysis should be populated in comprehensive mode")
	}

	if op.DataMigrationAnalysis == nil {
		t.Error("DataMigrationAnalysis should be populated in comprehensive mode")
	}

	if op.ConcurrencyAnalysis == nil {
		t.Error("ConcurrencyAnalysis should be populated in comprehensive mode")
	}
}
```

### Step 2: Run integration tests

Run: `go test ./tests/integration -v -run TestConcurrency`
Expected: PASS

### Step 3: Run all tests

Run: `go test ./... -v`
Expected: All tests PASS

### Step 4: Commit

```bash
git add tests/integration/advanced_features_test.go
git commit -m "test(concurrency): add integration tests for concurrency analysis"
```

---

## Task 8: Documentation

**Goal:** Document concurrency analysis feature usage

**Files:**
- Modify: `docs/advanced-features.md`
- Modify: `README.md`

### Step 1: Update advanced features documentation

Modify `docs/advanced-features.md`:

```markdown
## Concurrency Impact Analysis

Predict how migrations affect concurrent database operations and get recommendations for safer alternatives.

**Example Usage:**

```bash
# Analyze concurrency impact
tapa analyze migrations/ --db $DATABASE_URL --concurrency

# Include in comprehensive analysis
tapa analyze migrations/ --db $DATABASE_URL --comprehensive
```

**What it analyzes:**

- **Lock Types & Duration**: Predicts lock behavior and hold time
- **Blocked Queries**: Estimates number and types of queries affected
- **Current Workload**: Analyzes active connections and query patterns
- **Safer Alternatives**: Suggests concurrency-safe approaches
- **Downtime Estimation**: Predicts service disruption duration

**Example Output:**

```
Concurrency Impact:
─────────────────────────────────────
Impact Level: HIGH (score: 75/100)
Concurrency Safe: ✗ NO

Lock Details:
  Lock Type: ACCESS_EXCLUSIVE
  Duration: 30-60 seconds (45000 ms)
  Blocks: ALL operations (reads + writes)
  Estimated blocked queries: 150
  Lock acquisition risk: high

Current Workload:
  Active connections: 25
  Queries/second: 50.5
  Table access: high
  ⚠️  PEAK LOAD PERIOD
  Long-running queries: 3 (may delay lock)

Estimated Downtime: 45 seconds

Safer Alternatives:

  1. Add column without default, then backfill in batches
     Lock type: ACCESS_EXCLUSIVE
     Impact reduction: -80%
     Requires: Manual multi-step process
     Steps:
       - Step 1: ALTER TABLE ADD COLUMN (no default) - fast, brief lock
       - Step 2: UPDATE table SET column = value in batches - no table lock
       - Step 3: ALTER TABLE ALTER COLUMN SET DEFAULT - fast, brief lock
     Tradeoffs:
       • Requires manual batching logic
       • Column temporarily NULL during backfill
       • More complex deployment process

Recommendations:
  • ⚠️  HIGH IMPACT: This operation will block all queries for >30 seconds
  • Consider scheduling during maintenance window
  • ⚠️  Current workload is HIGH - consider delaying migration
  • Set statement_timeout to prevent indefinite blocking
  • Monitor active queries before executing: SELECT * FROM pg_stat_activity
```

**Impact Scoring:**

Concurrency impact is scored 0-100 based on:

1. **Lock Type** (0-40 points):
   - ACCESS EXCLUSIVE: 40 points (most restrictive)
   - EXCLUSIVE: 30 points
   - SHARE: 20 points
   - SHARE UPDATE EXCLUSIVE: 10 points (concurrent)
   - ROW EXCLUSIVE: 5 points (minimal impact)

2. **Lock Duration** (0-30 points):
   - >5 minutes: 30 points
   - >1 minute: 25 points
   - >30 seconds: 20 points
   - >10 seconds: 15 points
   - <10 seconds: 5-10 points

3. **Workload** (0-20 points):
   - Very high traffic: 20 points
   - High traffic: 15 points
   - Medium traffic: 10 points
   - Low traffic: 5 points

4. **Blocked Operations** (0-10 points):
   - Blocks reads + writes: 10 points
   - Blocks writes only: 5 points

**Impact Levels:**

- **MINIMAL** (0-20): Safe to run anytime
- **LOW** (21-40): Run during normal hours with monitoring
- **MEDIUM** (41-60): Run during low-traffic period
- **HIGH** (61-80): Coordinate with team, off-hours recommended
- **CRITICAL** (81-100): Maintenance window required

**Workload Analysis:**

When database connection is available, TAPA queries system tables to understand current load:

- **PostgreSQL**: Uses `pg_stat_activity`, `pg_stat_statements` (if available)
- **MySQL**: Uses `information_schema.processlist`, `performance_schema` (if enabled)

Workload metrics include:
- Active connections
- Queries per second to affected table
- Table access frequency
- Long-running queries (>5 seconds)
- Peak load detection

**Safer Alternatives:**

TAPA suggests concurrency-safe alternatives for common high-impact operations:

| Operation | Alternative | Impact Reduction |
|-----------|-------------|------------------|
| CREATE INDEX | CREATE INDEX CONCURRENTLY | 70% |
| ADD COLUMN with DEFAULT | Multi-step add + backfill | 80% |
| ALTER COLUMN type | New column + swap | 75% |
| DROP COLUMN | App change + delayed drop | 90% |

Each alternative includes:
- Step-by-step implementation guide
- Database version requirements
- Tradeoffs and considerations
- Estimated time impact

**Best Practices:**

1. **Run Analysis Before Migration**
   ```bash
   tapa analyze migration.sql --db $DATABASE_URL --concurrency
   ```

2. **Check During Off-Peak Hours**
   - Workload analysis is time-dependent
   - Re-run analysis during planned migration window

3. **Use Safer Alternatives for High Impact**
   - Score >60: Strongly consider alternatives
   - Score >80: Use alternatives or maintenance window

4. **Set Timeouts**
   ```sql
   SET statement_timeout = '60s';
   SET lock_timeout = '5s';
   ```

5. **Monitor Active Queries**
   ```sql
   -- PostgreSQL
   SELECT * FROM pg_stat_activity WHERE state = 'active';
   
   -- MySQL
   SHOW PROCESSLIST;
   ```

**Limitations:**

- Workload analysis requires database connection
- Estimates based on current load (may not reflect migration window)
- Cannot predict external factors (network issues, hardware failures)
- Some alternatives require manual implementation
- Lock acquisition can be delayed by existing queries

**Database Support:**

| Feature | PostgreSQL | MySQL |
|---------|------------|-------|
| Lock impact analysis | ✓ | ✓ |
| Workload analysis | ✓ | ✓ |
| Concurrent index creation | ✓ | Limited |
| Lock timeout | ✓ | ✓ |
| Statement timeout | ✓ | ✓ |
```

### Step 2: Update README

Modify `README.md`:

```markdown
### Advanced Features (Comprehensive Mode)

Enable advanced analysis:

```bash
# Full comprehensive analysis (all features)
tapa analyze migrations/ --db $DATABASE_URL --comprehensive

# Individual features
tapa analyze migrations/ --db $DATABASE_URL --dry-run
tapa analyze migrations/ --db $DATABASE_URL --concurrency
```

Features include:

- **Disk Space Requirements**: Calculate space needed before, during, and after migration
- **Rollback Analysis**: Determine reversibility and get auto-generated rollback scripts
- **Data Migration Detection**: Find hidden UPDATE/INSERT/DELETE operations with time estimates
- **Dry-Run Simulation**: Execute migrations in temporary schemas to catch runtime errors
- **Concurrency Impact Analysis**: Predict lock behavior and get safer alternatives

See [Advanced Features Guide](docs/advanced-features.md) for details.
```

### Step 3: Commit

```bash
git add docs/advanced-features.md README.md
git commit -m "docs(concurrency): add concurrency analysis documentation"
```

---

## Summary

This plan implements the Concurrency Impact Analysis feature (Feature 4) in 8 tasks:

1. **Models** - Define concurrency analysis data structures
2. **Lock Calculator** - Calculate lock impact based on operation type
3. **Workload Analyzer** - Analyze current database workload
4. **Analyzer** - Orchestrate concurrency analysis
5. **CLI Integration** - Add `--concurrency` flag
6. **Output Formatter** - Display concurrency analysis results
7. **Integration Test** - End-to-end verification
8. **Documentation** - Usage guide and best practices

**Key Features:**
- Predicts lock behavior and blocked queries
- Analyzes current workload patterns
- Scores concurrency impact (0-100)
- Suggests safer alternatives (CONCURRENTLY, multi-step)
- Provides actionable recommendations
- Works with PostgreSQL and MySQL

**Testing Strategy:**
- Unit tests for each component
- Integration tests with all advanced features
- Mock mode for tests without database
- Real workload analysis when DB available

**Backward Compatibility:**
- Feature is opt-in via `--concurrency` flag
- Included in `--comprehensive` mode
- No changes to existing analysis
- Gracefully handles missing database connection

**Completion:**
All 5 advanced features will be complete after this implementation:
✅ Feature 3: Disk Space Requirements
✅ Feature 2: Rollback Analysis
✅ Feature 5: Data Migration Detection
✅ Feature 1: Dry-Run Simulation
✅ Feature 4: Concurrency Impact Analysis
