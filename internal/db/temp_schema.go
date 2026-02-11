package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

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
	// Generate unique schema name with timestamp
	timestamp := time.Now().Unix()
	schemaName := fmt.Sprintf("tapa_temp_%d", timestamp)

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

	// Clone each table structure (without data)
	for _, table := range tables {
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

	// Clone each table
	for _, table := range tables {
		if len(includeTables) > 0 && !contains(includeTables, table) {
			continue
		}

		// Get CREATE TABLE statement
		var createTableSQL string
		err := db.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE TABLE %s.%s", sourceSchema, table)).Scan(&table, &createTableSQL)
		if err != nil {
			return fmt.Errorf("failed to get CREATE TABLE for %s: %w", table, err)
		}

		// Switch to temp database and create table
		if _, err := db.ExecContext(ctx, fmt.Sprintf("USE %s", tempSchema)); err != nil {
			return err
		}

		if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
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
