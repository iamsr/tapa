package mysql

import (
	"strings"
	"testing"

	"github.com/yourusername/dma/pkg/models"
)

func TestGeneratePtOscCommand(t *testing.T) {
	op := &models.Operation{
		Type:       models.OperationTypeAddColumn,
		TableName:  "users",
		ColumnName: "email",
		SQL:        "ALTER TABLE users ADD COLUMN email VARCHAR(255);",
		RiskScore:  75,
	}

	cmd := GeneratePtOscCommand(op, "localhost", "mydb")

	if cmd == "" {
		t.Error("Expected pt-osc command, got empty string")
	}

	if !strings.Contains(cmd, "pt-online-schema-change") {
		t.Error("Command should start with pt-online-schema-change")
	}

	if !strings.Contains(cmd, "--alter") {
		t.Error("Command should include --alter flag")
	}
}

func TestGeneratePtOscCommand_LowRisk(t *testing.T) {
	op := &models.Operation{
		Type:       models.OperationTypeAddColumn,
		TableName:  "users",
		ColumnName: "status",
		SQL:        "ALTER TABLE users ADD COLUMN status VARCHAR(50);",
		RiskScore:  30, // Low risk
	}

	cmd := GeneratePtOscCommand(op, "localhost", "mydb")

	if cmd != "" {
		t.Errorf("Expected empty string for low risk operation, got: %s", cmd)
	}
}

func TestShouldUsePtOsc(t *testing.T) {
	tests := []struct {
		name     string
		op       *models.Operation
		expected bool
	}{
		{
			name: "high risk add column with rewrite",
			op: &models.Operation{
				Type:            models.OperationTypeAddColumn,
				RiskScore:       75,
				RequiresRewrite: true,
			},
			expected: true,
		},
		{
			name: "low risk operation",
			op: &models.Operation{
				Type:            models.OperationTypeAddColumn,
				RiskScore:       30,
				RequiresRewrite: true,
			},
			expected: false,
		},
		{
			name: "high risk but no rewrite needed",
			op: &models.Operation{
				Type:            models.OperationTypeAddColumn,
				RiskScore:       75,
				RequiresRewrite: false,
			},
			expected: false,
		},
		{
			name: "high risk drop column with rewrite",
			op: &models.Operation{
				Type:            models.OperationTypeDropColumn,
				RiskScore:       80,
				RequiresRewrite: true,
			},
			expected: true,
		},
		{
			name: "high risk create index (not ALTER TABLE)",
			op: &models.Operation{
				Type:            models.OperationTypeCreateIndex,
				RiskScore:       75,
				RequiresRewrite: true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldUsePtOsc(tt.op)
			if result != tt.expected {
				t.Errorf("ShouldUsePtOsc() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractAlterClause(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "add column",
			sql:      "ALTER TABLE users ADD COLUMN email VARCHAR(255);",
			expected: "ADD COLUMN email VARCHAR(255)",
		},
		{
			name:     "drop column",
			sql:      "ALTER TABLE products DROP COLUMN price;",
			expected: "DROP COLUMN price",
		},
		{
			name:     "modify column",
			sql:      "ALTER TABLE orders MODIFY COLUMN status VARCHAR(100);",
			expected: "MODIFY COLUMN status VARCHAR(100)",
		},
		{
			name:     "no semicolon",
			sql:      "ALTER TABLE items ADD COLUMN quantity INT",
			expected: "ADD COLUMN quantity INT",
		},
		{
			name:     "invalid SQL - no ALTER TABLE",
			sql:      "CREATE TABLE users (id INT);",
			expected: "",
		},
		{
			name:     "invalid SQL - only ALTER TABLE",
			sql:      "ALTER TABLE",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAlterClause(tt.sql)
			if result != tt.expected {
				t.Errorf("extractAlterClause() = %q, want %q", result, tt.expected)
			}
		})
	}
}
