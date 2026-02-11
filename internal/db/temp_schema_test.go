package db_test

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/internal/db"
)

func TestTempSchemaCreator_CreateSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	// This test requires a running PostgreSQL database
	// Skip if DATABASE_URL not set
	creator := db.NewTempSchemaCreator("postgresql")

	ctx := context.Background()
	schemaName, cleanup, err := creator.CreateSchema(ctx, nil)
	if err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}
	defer cleanup(ctx)

	if schemaName == "" {
		t.Error("Schema name should not be empty")
	}

	// Schema name should start with "tapa_temp_"
	if len(schemaName) < 10 || schemaName[:10] != "tapa_temp_" {
		t.Errorf("Schema name should start with 'tapa_temp_', got %s", schemaName)
	}
}
