package parser

import (
	"fmt"

	"github.com/yourusername/dma/internal/parser/postgres"
	"github.com/yourusername/dma/pkg/models"
)

// Parser is the interface for database-specific SQL parsers
type Parser interface {
	// Parse analyzes SQL and returns detected operations
	Parse(sql string) ([]*models.Operation, error)

	// ParseFile reads and parses a migration file
	ParseFile(filePath string) (*models.Migration, error)
}

// GetParser returns the appropriate parser for the database type
func GetParser(dbType string) (Parser, error) {
	switch dbType {
	case "postgresql":
		return postgres.NewParser(), nil
	case "mysql":
		// TODO: Implement MySQL parser in Phase 3
		return nil, fmt.Errorf("MySQL parser not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
