package models

import "time"

// Migration represents a database migration file
type Migration struct {
	FilePath   string
	Operations []*Operation
	ParsedAt   time.Time
}

// NewMigration creates a new Migration
func NewMigration(filePath string) *Migration {
	return &Migration{
		FilePath:   filePath,
		Operations: make([]*Operation, 0),
		ParsedAt:   time.Now(),
	}
}

// AddOperation adds an operation to the migration
func (m *Migration) AddOperation(op *Operation) {
	m.Operations = append(m.Operations, op)
}

// MaxRiskScore returns the highest risk score among all operations
func (m *Migration) MaxRiskScore() int {
	max := 0
	for _, op := range m.Operations {
		if op.RiskScore > max {
			max = op.RiskScore
		}
	}
	return max
}

// HasHighRisk returns true if any operation has risk score >= 51
func (m *Migration) HasHighRisk() bool {
	return m.MaxRiskScore() >= 51
}
