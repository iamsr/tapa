package parser

import (
	"fmt"

	"github.com/iamsr/dma/internal/parser/mysql"
	"github.com/iamsr/dma/internal/parser/postgres"
	"github.com/iamsr/dma/pkg/models"
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
		return mysql.NewParser(), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
