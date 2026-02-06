package mysql

import (
	"context"
	"testing"

	"github.com/iamsr/dma/pkg/models"
)

func TestAnalyzer_Integration_ComplexMigration(t *testing.T) {
	// Integration test without real database
	analyzer := NewAnalyzer(nil, 100, 2.0)

	tests := []struct {
		name         string
		sql          string
		opType       models.OperationType
		expectedLock models.LockType
		expectedRisk int
		minRecs      int
	}{
		{
			name:         "ADD COLUMN without DEFAULT",
			sql:          "ALTER TABLE users ADD COLUMN email VARCHAR(255);",
			opType:       models.OperationTypeAddColumn,
			expectedLock: models.LockTypeNone,
			expectedRisk: 0,
			minRecs:      0,
		},
		{
			name:         "ADD COLUMN with DEFAULT",
			sql:          "ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT 'unknown';",
			opType:       models.OperationTypeAddColumn,
			expectedLock: models.LockTypeExclusive,
			expectedRisk: 40,
			minRecs:      2,
		},
		{
			name:         "CREATE INDEX online",
			sql:          "CREATE INDEX idx_email ON users(email) ALGORITHM=INPLACE;",
			opType:       models.OperationTypeCreateIndex,
			expectedLock: models.LockTypeNone,
			expectedRisk: 0,
			minRecs:      1,
		},
		{
			name:         "DROP COLUMN",
			sql:          "ALTER TABLE users DROP COLUMN email;",
			opType:       models.OperationTypeDropColumn,
			expectedLock: models.LockTypeNone,
			expectedRisk: 10,
			minRecs:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &models.Operation{
				Type:      tt.opType,
				TableName: "users",
				SQL:       tt.sql,
			}

			err := analyzer.Analyze(context.Background(), op)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if op.LockType != tt.expectedLock {
				t.Errorf("Expected lock %s, got %s", tt.expectedLock, op.LockType)
			}

			if op.RiskScore < tt.expectedRisk {
				t.Errorf("Expected risk >= %d, got %d", tt.expectedRisk, op.RiskScore)
			}

			if len(op.Recommendations) < tt.minRecs {
				t.Errorf("Expected >= %d recommendations, got %d", tt.minRecs, len(op.Recommendations))
			}
		})
	}
}
