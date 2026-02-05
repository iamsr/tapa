package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMigration_AddOperation(t *testing.T) {
	m := NewMigration("001_test.sql")

	op := &Operation{
		SQL:  "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
		Type: OperationTypeAlterTable,
	}

	m.AddOperation(op)

	assert.Equal(t, 1, len(m.Operations))
	assert.Equal(t, op, m.Operations[0])
}

func TestMigration_HasHighRisk(t *testing.T) {
	m := NewMigration("001_test.sql")
	m.AddOperation(&Operation{RiskScore: 80})

	assert.True(t, m.HasHighRisk())

	m2 := NewMigration("002_test.sql")
	m2.AddOperation(&Operation{RiskScore: 20})

	assert.False(t, m2.HasHighRisk())
}
