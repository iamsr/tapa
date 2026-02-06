package mysql

import (
	"context"
	"testing"

	"github.com/iamsr/tapa/pkg/models"
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

func TestAnalyzer_AnalyzeWithEnhancements_Phase2(t *testing.T) {
	// Integration test for Phase 2 features (dependencies, time estimation, alternatives)
	analyzer := NewAnalyzer(nil, 100, 2.0)
	ctx := context.Background()

	tests := []struct {
		name                 string
		sql                  string
		opType               models.OperationType
		expectDependencies   bool
		expectTimeBreakdown  bool
		expectAlternatives   bool
		includeDependencies  bool
		includeTimeBreakdown bool
		includeAlternatives  bool
	}{
		{
			name:                 "ADD COLUMN with all enhancements",
			sql:                  "ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT 'unknown';",
			opType:               models.OperationTypeAddColumn,
			expectDependencies:   false, // No dependencies without introspector
			expectTimeBreakdown:  true,  // Should have time estimate
			expectAlternatives:   false, // ADD COLUMN doesn't have alternatives
			includeDependencies:  true,
			includeTimeBreakdown: true,
			includeAlternatives:  true,
		},
		{
			name:                 "CREATE INDEX with time breakdown",
			sql:                  "CREATE INDEX idx_email ON users(email);",
			opType:               models.OperationTypeCreateIndex,
			expectDependencies:   false,
			expectTimeBreakdown:  true,
			expectAlternatives:   false, // Won't generate alternatives without high risk score
			includeDependencies:  false,
			includeTimeBreakdown: true,
			includeAlternatives:  true,
		},
		{
			name:                 "DROP COLUMN with dependencies only",
			sql:                  "ALTER TABLE users DROP COLUMN email;",
			opType:               models.OperationTypeDropColumn,
			expectDependencies:   false, // No dependencies without introspector
			expectTimeBreakdown:  false,
			expectAlternatives:   false,
			includeDependencies:  true,
			includeTimeBreakdown: false,
			includeAlternatives:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &models.Operation{
				Type:      tt.opType,
				TableName: "users",
				SQL:       tt.sql,
			}

			opts := AnalysisOptions{
				IncludeDependencies:  tt.includeDependencies,
				IncludeTimeBreakdown: tt.includeTimeBreakdown,
				IncludeAlternatives:  tt.includeAlternatives,
			}

			err := analyzer.AnalyzeWithEnhancements(ctx, op, opts)
			if err != nil {
				t.Fatalf("AnalyzeWithEnhancements failed: %v", err)
			}

			// Basic analysis should still work
			if op.LockType == "" {
				t.Error("Expected lock type to be set")
			}

			// Check Phase 2 features
			if tt.expectDependencies && len(op.Dependencies) == 0 {
				t.Error("Expected dependencies to be populated")
			}

			if tt.expectTimeBreakdown && op.TimeBreakdown == nil {
				t.Error("Expected time breakdown to be populated")
			}

			if tt.expectAlternatives && len(op.Alternatives) == 0 {
				t.Error("Expected alternatives to be populated")
			}

			// Verify time breakdown structure if present
			if op.TimeBreakdown != nil {
				if op.TimeBreakdown.TotalSeconds == 0 {
					t.Error("Expected total time to be calculated")
				}
			}
		})
	}
}
