package db

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// validateIdentifier ensures identifier is safe for SQL interpolation
func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	// Allow alphanumeric and underscore, must start with letter or underscore
	if !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(name) {
		return fmt.Errorf("invalid identifier: %s (must contain only letters, numbers, and underscores)", name)
	}
	return nil
}

// TempSchemaCreator creates temporary isolated schemas for dry-run testing
type TempSchemaCreator struct {
	databaseType string
}

// NewTempSchemaCreator creates a new temporary schema creator
func NewTempSchemaCreator(databaseType string) *TempSchemaCreator {
	return &TempSchemaCreator{
		databaseType: databaseType,
	}
}

// CreateSchema creates a temporary schema and returns its name and cleanup function
func (c *TempSchemaCreator) CreateSchema(ctx context.Context, db *sql.DB) (string, func(context.Context) error, error) {
	// Generate unique schema name with timestamp and random component
	timestamp := time.Now().Unix()
	random := rand.Intn(100000) // 5-digit random number
	schemaName := fmt.Sprintf("tapa_temp_%d_%05d", timestamp, random)

	// Validate schema name
	if err := validateIdentifier(schemaName); err != nil {
		return "", nil, fmt.Errorf("invalid schema name: %w", err)
	}

	switch c.databaseType {
	case "postgresql":
		return c.createPostgreSQLSchema(ctx, db, schemaName)
	case "mysql":
		return c.createMySQLSchema(ctx, db, schemaName)
	default:
		return "", nil, fmt.Errorf("unsupported database type: %s", c.databaseType)
	}
}

func (c *TempSchemaCreator) createPostgreSQLSchema(ctx context.Context, db *sql.DB, schemaName string) (string, func(context.Context) error, error) {
	if db == nil {
		// Mock mode for testing without database
		cleanup := func(ctx context.Context) error { return nil }
		return schemaName, cleanup, nil
	}

	// Create schema
	_, err := db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Cleanup function to drop schema
	cleanup := func(ctx context.Context) error {
		_, err := db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		return err
	}

	return schemaName, cleanup, nil
}

func (c *TempSchemaCreator) createMySQLSchema(ctx context.Context, db *sql.DB, schemaName string) (string, func(context.Context) error, error) {
	if db == nil {
		cleanup := func(ctx context.Context) error { return nil }
		return schemaName, cleanup, nil
	}

	// MySQL uses databases instead of schemas
	_, err := db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", schemaName))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create database: %w", err)
	}

	cleanup := func(ctx context.Context) error {
		_, err := db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", schemaName))
		return err
	}

	return schemaName, cleanup, nil
}

// CloneSchema copies schema structure from source to temp schema
func (c *TempSchemaCreator) CloneSchema(ctx context.Context, db *sql.DB, sourceSchema, tempSchema string, includeTables []string) error {
	if db == nil {
		return nil // Mock mode
	}

	// Validate schema names
	if err := validateIdentifier(sourceSchema); err != nil {
		return fmt.Errorf("invalid source schema: %w", err)
	}
	if err := validateIdentifier(tempSchema); err != nil {
		return fmt.Errorf("invalid temp schema: %w", err)
	}

	switch c.databaseType {
	case "postgresql":
		return c.clonePostgreSQLSchema(ctx, db, sourceSchema, tempSchema, includeTables)
	case "mysql":
		return c.cloneMySQLSchema(ctx, db, sourceSchema, tempSchema, includeTables)
	default:
		return fmt.Errorf("unsupported database type: %s", c.databaseType)
	}
}

func (c *TempSchemaCreator) clonePostgreSQLSchema(ctx context.Context, db *sql.DB, sourceSchema, tempSchema string, includeTables []string) error {
	// Get table definitions from source schema
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1
		AND table_type = 'BASE TABLE'
	`

	rows, err := db.QueryContext(ctx, query, sourceSchema)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		tables = append(tables, tableName)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating tables: %w", err)
	}

	// Clone each table structure (without data)
	for _, table := range tables {
		// Validate table name
		if err := validateIdentifier(table); err != nil {
			return fmt.Errorf("invalid table name %s: %w", table, err)
		}

		// Skip if table not in include list (if specified)
		if len(includeTables) > 0 && !contains(includeTables, table) {
			continue
		}

		// Create table in temp schema with same structure
		cloneSQL := fmt.Sprintf(
			"CREATE TABLE %s.%s (LIKE %s.%s INCLUDING ALL)",
			tempSchema, table, sourceSchema, table,
		)

		if _, err := db.ExecContext(ctx, cloneSQL); err != nil {
			return fmt.Errorf("failed to clone table %s: %w", table, err)
		}
	}

	return nil
}

func (c *TempSchemaCreator) cloneMySQLSchema(ctx context.Context, db *sql.DB, sourceSchema, tempSchema string, includeTables []string) error {
	// Get table definitions
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ?
		AND table_type = 'BASE TABLE'
	`

	rows, err := db.QueryContext(ctx, query, sourceSchema)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		tables = append(tables, tableName)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating tables: %w", err)
	}

	// Clone each table
	for _, table := range tables {
		// Validate table name
		if err := validateIdentifier(table); err != nil {
			return fmt.Errorf("invalid table name %s: %w", table, err)
		}

		if len(includeTables) > 0 && !contains(includeTables, table) {
			continue
		}

		// Get CREATE TABLE statement
		var tableName, createTableSQL string
		err := db.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE TABLE %s.%s", sourceSchema, table)).Scan(&tableName, &createTableSQL)
		if err != nil {
			return fmt.Errorf("failed to get CREATE TABLE for %s: %w", table, err)
		}

		// Modify CREATE TABLE to use qualified table name instead of USE
		// Change: CREATE TABLE `tablename` to CREATE TABLE `tempSchema`.`tablename`
		createSQL := strings.Replace(createTableSQL,
			"CREATE TABLE `"+table+"`",
			"CREATE TABLE `"+tempSchema+"`.`"+table+"`", 1)

		if _, err := db.ExecContext(ctx, createSQL); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table, err)
		}
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
