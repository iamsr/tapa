package introspector

import (
	"fmt"

	"github.com/yourusername/dma/internal/db"
	"github.com/yourusername/dma/internal/db/postgres"
)

// GetIntrospector returns the appropriate introspector for the database type
func GetIntrospector(dbType, connURL string) (db.Introspector, error) {
	switch dbType {
	case "postgresql":
		return postgres.NewIntrospector(connURL), nil
	case "mysql":
		return nil, fmt.Errorf("MySQL introspector not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
