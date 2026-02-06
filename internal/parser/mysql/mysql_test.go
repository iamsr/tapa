package mysql

import (
	"os"
	"testing"

	"github.com/yourusername/dma/pkg/models"
)

func TestParser_Parse_CreateTable(t *testing.T) {
	parser := NewParser()
	sql := "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(255));"

	ops, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != models.OperationTypeCreateTable {
		t.Errorf("Expected CREATE_TABLE, got %s", op.Type)
	}
	if op.TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", op.TableName)
	}
}

func TestParser_ParseFile_MultipleStatements(t *testing.T) {
	parser := NewParser()

	// Create temp file with multiple statements
	tmpFile, err := os.CreateTemp("", "test_*.sql")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	sql := `CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(255));
CREATE TABLE posts (id INT PRIMARY KEY, user_id INT);`

	if _, err := tmpFile.Write([]byte(sql)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	migration, err := parser.ParseFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(migration.Operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(migration.Operations))
	}

	// Verify both tables parsed correctly
	if migration.Operations[0].TableName != "users" {
		t.Errorf("Expected first table 'users', got '%s'", migration.Operations[0].TableName)
	}
	if migration.Operations[1].TableName != "posts" {
		t.Errorf("Expected second table 'posts', got '%s'", migration.Operations[1].TableName)
	}
}
