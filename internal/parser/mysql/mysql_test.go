package mysql

import (
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
