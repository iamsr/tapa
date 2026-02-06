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

func TestParser_Parse_AddColumn(t *testing.T) {
	parser := NewParser()
	sql := "ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT 'unknown';"

	ops, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != models.OperationTypeAddColumn {
		t.Errorf("Expected ADD_COLUMN, got %s", op.Type)
	}
	if op.TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", op.TableName)
	}
	if op.ColumnName != "email" {
		t.Errorf("Expected column 'email', got '%s'", op.ColumnName)
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

func TestParser_Parse_DropColumn(t *testing.T) {
	parser := NewParser()
	sql := "ALTER TABLE users DROP COLUMN email;"

	ops, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != models.OperationTypeDropColumn {
		t.Errorf("Expected DROP_COLUMN, got %s", op.Type)
	}
	if op.TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", op.TableName)
	}
	if op.ColumnName != "email" {
		t.Errorf("Expected column 'email', got '%s'", op.ColumnName)
	}
}

func TestParser_Parse_AlterColumn(t *testing.T) {
	parser := NewParser()
	sql := "ALTER TABLE users MODIFY COLUMN email TEXT;"

	ops, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != models.OperationTypeAlterColumn {
		t.Errorf("Expected ALTER_COLUMN, got %s", op.Type)
	}
	if op.TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", op.TableName)
	}
	if op.ColumnName != "email" {
		t.Errorf("Expected column 'email', got '%s'", op.ColumnName)
	}
}

func TestParser_Parse_CreateIndex(t *testing.T) {
	parser := NewParser()
	sql := "CREATE INDEX idx_email ON users(email);"

	ops, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != models.OperationTypeCreateIndex {
		t.Errorf("Expected CREATE_INDEX, got %s", op.Type)
	}
	if op.TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", op.TableName)
	}
	if op.IndexName != "idx_email" {
		t.Errorf("Expected index 'idx_email', got '%s'", op.IndexName)
	}
}

func TestParser_Parse_DropIndex(t *testing.T) {
	parser := NewParser()
	sql := "DROP INDEX idx_email ON users;"

	ops, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != models.OperationTypeDropIndex {
		t.Errorf("Expected DROP_INDEX, got %s", op.Type)
	}
	if op.TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", op.TableName)
	}
	if op.IndexName != "idx_email" {
		t.Errorf("Expected index 'idx_email', got '%s'", op.IndexName)
	}
}
