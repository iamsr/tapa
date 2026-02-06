package batcher

import (
	"fmt"

	"github.com/iamsr/dma/pkg/models"
)

// MigrationBatcher generates batching strategies for migration operations
type MigrationBatcher interface {
	// GenerateBatches groups operations by risk level and creates a batching strategy
	GenerateBatches(ops []*models.Operation) (*models.BatchingStrategy, error)
}

// GetMigrationBatcher returns appropriate batcher for database type
func GetMigrationBatcher(dbType string) (MigrationBatcher, error) {
	switch dbType {
	case "postgresql":
		return newPostgresBatcher(), nil
	case "mysql":
		return NewMySQLBatcher(), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// postgresBatcher implements MigrationBatcher for PostgreSQL
type postgresBatcher struct{}

func newPostgresBatcher() *postgresBatcher {
	return &postgresBatcher{}
}

// GenerateBatches groups operations by risk level
func (b *postgresBatcher) GenerateBatches(ops []*models.Operation) (*models.BatchingStrategy, error) {
	strategy := &models.BatchingStrategy{
		Batches: []models.MigrationBatch{},
	}

	// Handle empty operations
	if len(ops) == 0 {
		return strategy, nil
	}

	// Group operations by risk level
	var lowRisk []*models.Operation
	var mediumRisk []*models.Operation
	var highRisk []*models.Operation

	for _, op := range ops {
		switch {
		case op.RiskScore < 26:
			lowRisk = append(lowRisk, op)
		case op.RiskScore < 51:
			mediumRisk = append(mediumRisk, op)
		default:
			highRisk = append(highRisk, op)
		}
	}

	batchNumber := 1
	var allBatchNumbers []int

	// Batch 1: Low-risk operations (can run in parallel)
	if len(lowRisk) > 0 {
		batch := models.MigrationBatch{
			BatchNumber:      batchNumber,
			Operations:       lowRisk,
			CanRunInParallel: true,
			Prerequisites:    []int{},
			Rationale:        "Low-risk operations can be deployed immediately",
		}
		batch.CalculateMetrics()
		strategy.Batches = append(strategy.Batches, batch)
		allBatchNumbers = append(allBatchNumbers, batchNumber)
		batchNumber++
	}

	// Batch 2: Medium-risk operations (sequential, depends on low-risk)
	if len(mediumRisk) > 0 {
		prerequisites := make([]int, len(allBatchNumbers))
		copy(prerequisites, allBatchNumbers)

		batch := models.MigrationBatch{
			BatchNumber:      batchNumber,
			Operations:       mediumRisk,
			CanRunInParallel: false,
			Prerequisites:    prerequisites,
			Rationale:        "Medium-risk operations should be deployed during low-traffic periods",
		}
		batch.CalculateMetrics()
		strategy.Batches = append(strategy.Batches, batch)
		allBatchNumbers = append(allBatchNumbers, batchNumber)
		batchNumber++
	}

	// Batches 3+: One high/critical risk operation per batch
	for _, op := range highRisk {
		prerequisites := make([]int, len(allBatchNumbers))
		copy(prerequisites, allBatchNumbers)

		batch := models.MigrationBatch{
			BatchNumber:      batchNumber,
			Operations:       []*models.Operation{op},
			CanRunInParallel: false,
			Prerequisites:    prerequisites,
			Rationale:        "High-risk operation requires maintenance window",
		}
		batch.CalculateMetrics()
		strategy.Batches = append(strategy.Batches, batch)
		allBatchNumbers = append(allBatchNumbers, batchNumber)
		batchNumber++
	}

	// Calculate metrics for all batches
	strategy.CalculateMetrics()

	// Generate recommendations
	strategy.Recommendations = b.generateRecommendations(strategy)

	return strategy, nil
}

// generateRecommendations creates recommendations based on batching strategy
func (b *postgresBatcher) generateRecommendations(strategy *models.BatchingStrategy) []string {
	recs := []string{}

	if strategy.TotalBatches > 1 {
		recs = append(recs,
			fmt.Sprintf("💡 Split into %d batches for safer deployment", strategy.TotalBatches))
	}

	for _, batch := range strategy.Batches {
		if batch.RiskLevel == models.RiskLevelCritical || batch.RiskLevel == models.RiskLevelHigh {
			recs = append(recs,
				fmt.Sprintf("⚠️  Batch %d (%s): Deploy during maintenance window", batch.BatchNumber, batch.RiskLevel))
		}
	}

	return recs
}

// MySQLBatcher implements MigrationBatcher for MySQL
type MySQLBatcher struct{}

// NewMySQLBatcher creates a MySQL batcher
func NewMySQLBatcher() *MySQLBatcher {
	return &MySQLBatcher{}
}

// GenerateBatches groups operations by risk level (same logic as PostgreSQL)
func (b *MySQLBatcher) GenerateBatches(operations []*models.Operation) (*models.BatchingStrategy, error) {
	strategy := &models.BatchingStrategy{
		Batches: []models.MigrationBatch{},
	}

	// Handle empty operations
	if len(operations) == 0 {
		return strategy, nil
	}

	// Group operations by risk level
	var lowRisk []*models.Operation
	var mediumRisk []*models.Operation
	var highRisk []*models.Operation

	for _, op := range operations {
		level := op.RiskLevel()
		switch level {
		case models.RiskLevelLow:
			lowRisk = append(lowRisk, op)
		case models.RiskLevelMedium:
			mediumRisk = append(mediumRisk, op)
		case models.RiskLevelHigh, models.RiskLevelCritical:
			highRisk = append(highRisk, op)
		}
	}

	batchNumber := 1
	var allBatchNumbers []int

	// Batch 1: Low-risk operations (can run in parallel)
	if len(lowRisk) > 0 {
		batch := models.MigrationBatch{
			BatchNumber:      batchNumber,
			Operations:       lowRisk,
			CanRunInParallel: true,
			Prerequisites:    []int{},
			Rationale:        "Low risk operations can be executed together",
		}
		batch.CalculateMetrics()
		strategy.Batches = append(strategy.Batches, batch)
		allBatchNumbers = append(allBatchNumbers, batchNumber)
		batchNumber++
	}

	// Batch 2: Medium-risk operations (sequential, depends on low-risk)
	if len(mediumRisk) > 0 {
		prerequisites := make([]int, len(allBatchNumbers))
		copy(prerequisites, allBatchNumbers)

		batch := models.MigrationBatch{
			BatchNumber:      batchNumber,
			Operations:       mediumRisk,
			CanRunInParallel: false,
			Prerequisites:    prerequisites,
			Rationale:        "Medium risk operations - execute sequentially with monitoring",
		}
		batch.CalculateMetrics()
		strategy.Batches = append(strategy.Batches, batch)
		allBatchNumbers = append(allBatchNumbers, batchNumber)
		batchNumber++
	}

	// Batches 3+: One high/critical risk operation per batch
	for _, op := range highRisk {
		prerequisites := make([]int, len(allBatchNumbers))
		copy(prerequisites, allBatchNumbers)

		batch := models.MigrationBatch{
			BatchNumber:      batchNumber,
			Operations:       []*models.Operation{op},
			CanRunInParallel: false,
			Prerequisites:    prerequisites,
			Rationale:        "High risk operation - isolate and monitor closely",
		}
		batch.CalculateMetrics()
		strategy.Batches = append(strategy.Batches, batch)
		allBatchNumbers = append(allBatchNumbers, batchNumber)
		batchNumber++
	}

	// Calculate metrics for all batches
	strategy.CalculateMetrics()

	return strategy, nil
}
