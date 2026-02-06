package introspector

import (
	"fmt"

	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/internal/db/mysql"
	"github.com/iamsr/tapa/internal/db/postgres"
)

// GetIntrospector returns the appropriate introspector for the database type
func GetIntrospector(dbType, connURL string) (db.Introspector, error) {
	switch dbType {
	case "postgresql":
		return postgres.NewIntrospector(connURL), nil
	case "mysql":
		return mysql.NewIntrospector(connURL), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
