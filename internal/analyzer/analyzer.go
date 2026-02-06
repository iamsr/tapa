package analyzer

import (
	"context"
	"fmt"

	mysqlanalyzer "github.com/yourusername/dma/internal/analyzer/mysql"
	postgresanalyzer "github.com/yourusername/dma/internal/analyzer/postgres"
	"github.com/yourusername/dma/internal/db"
	"github.com/yourusername/dma/pkg/models"
)

// Analyzer analyzes database operations for production impact
type Analyzer interface {
	// Analyze enriches an operation with lock detection, risk scoring, and recommendations
	Analyze(ctx context.Context, op *models.Operation) error
}

// GetAnalyzer returns the appropriate analyzer for the database type
func GetAnalyzer(dbType string, introspector db.Introspector, diskThroughputMBps int, rewriteFactor float64) (Analyzer, error) {
	switch dbType {
	case "postgresql":
		return postgresanalyzer.NewAnalyzer(introspector, diskThroughputMBps, rewriteFactor), nil
	case "mysql":
		return mysqlanalyzer.NewAnalyzer(introspector, diskThroughputMBps, rewriteFactor), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
