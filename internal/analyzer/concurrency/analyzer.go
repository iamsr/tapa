package concurrency

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iamsr/tapa/pkg/models"
)

// Analyzer performs concurrency impact analysis
type Analyzer struct {
	databaseType     string
	db               *sql.DB
	lockCalculator   *LockCalculator
	workloadAnalyzer *WorkloadAnalyzer
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
