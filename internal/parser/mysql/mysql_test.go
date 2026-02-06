package mysql

import (
	"os"
	"testing"

	"github.com/iamsr/dma/pkg/models"
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
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
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
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
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
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
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
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
	}
}

func TestParser_Parse_ChangeColumn(t *testing.T) {
	parser := NewParser()
	sql := "ALTER TABLE users CHANGE COLUMN old_email new_email VARCHAR(255);"

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
	if op.ColumnName != "new_email" {
		t.Errorf("Expected column 'new_email', got '%s'", op.ColumnName)
	}
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
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
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
	}
}

func TestParser_Parse_AddIndexViaAlterTable(t *testing.T) {
	parser := NewParser()
	sql := "ALTER TABLE users ADD INDEX idx_email(email);"

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
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
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
	if op.SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, op.SQL)
	}
}

func TestParser_Parse_MultipleAlterOperations(t *testing.T) {
	parser := NewParser()
	sql := "ALTER TABLE users ADD COLUMN x INT, DROP COLUMN y, ADD INDEX idx(x);"

	ops, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(ops) != 3 {
		t.Fatalf("Expected 3 operations, got %d", len(ops))
	}

	// Verify first operation: ADD COLUMN
	if ops[0].Type != models.OperationTypeAddColumn {
		t.Errorf("Expected first operation to be ADD_COLUMN, got %s", ops[0].Type)
	}
	if ops[0].TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", ops[0].TableName)
	}
	if ops[0].ColumnName != "x" {
		t.Errorf("Expected column 'x', got '%s'", ops[0].ColumnName)
	}
	if ops[0].SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, ops[0].SQL)
	}

	// Verify second operation: DROP COLUMN
	if ops[1].Type != models.OperationTypeDropColumn {
		t.Errorf("Expected second operation to be DROP_COLUMN, got %s", ops[1].Type)
	}
	if ops[1].TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", ops[1].TableName)
	}
	if ops[1].ColumnName != "y" {
		t.Errorf("Expected column 'y', got '%s'", ops[1].ColumnName)
	}
	if ops[1].SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, ops[1].SQL)
	}

	// Verify third operation: CREATE INDEX
	if ops[2].Type != models.OperationTypeCreateIndex {
		t.Errorf("Expected third operation to be CREATE_INDEX, got %s", ops[2].Type)
	}
	if ops[2].TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", ops[2].TableName)
	}
	if ops[2].IndexName != "idx" {
		t.Errorf("Expected index 'idx', got '%s'", ops[2].IndexName)
	}
	if ops[2].SQL != sql {
		t.Errorf("Expected SQL %q, got %q", sql, ops[2].SQL)
	}
}
