package alternatives

import (
	"context"
	"strings"
	"testing"

	"github.com/yourusername/dma/pkg/models"
)

func TestGetAlternativeGenerator(t *testing.T) {
	tests := []struct {
		name      string
		dbType    string
		wantErr   bool
		checkType func(AlternativeGenerator) bool
	}{
		{
			name:    "postgresql returns postgres generator",
			dbType:  "postgresql",
			wantErr: false,
			checkType: func(g AlternativeGenerator) bool {
				_, ok := g.(*postgresGenerator)
				return ok
			},
		},
		{
			name:    "postgres returns postgres generator",
			dbType:  "postgres",
			wantErr: false,
			checkType: func(g AlternativeGenerator) bool {
				_, ok := g.(*postgresGenerator)
				return ok
			},
		},
		{
			name:    "mysql returns error - not supported",
			dbType:  "mysql",
			wantErr: true,
		},
		{
			name:    "unsupported database returns error",
			dbType:  "mongodb",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := GetAlternativeGenerator(tt.dbType)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetAlternativeGenerator() expected error but got none")
				}
				if gen != nil {
					t.Errorf("GetAlternativeGenerator() expected nil generator on error, got %T", gen)
				}
			} else {
				if err != nil {
					t.Errorf("GetAlternativeGenerator() unexpected error: %v", err)
				}
				if gen == nil {
					t.Errorf("GetAlternativeGenerator() expected generator but got nil")
				}
				if tt.checkType != nil && !tt.checkType(gen) {
					t.Errorf("GetAlternativeGenerator() returned wrong generator type: %T", gen)
				}
			}
		})
	}
}

func TestPostgresGenerator_CanGenerateAlternative(t *testing.T) {
	gen, err := GetAlternativeGenerator("postgresql")
	if err != nil {
		t.Fatalf("Failed to get generator: %v", err)
	}

	tests := []struct {
		name string
		op   *models.Operation
		want bool
	}{
		{
			name: "low risk operation returns false",
			op: &models.Operation{
				SQL:       "ALTER TABLE users ADD COLUMN age INT DEFAULT 0",
				RiskScore: 50,
			},
			want: false,
		},
		{
			name: "high risk ADD COLUMN with DEFAULT returns true",
			op: &models.Operation{
				SQL:       "ALTER TABLE users ADD COLUMN age INT DEFAULT 0",
				Type:      models.OperationTypeAddColumn,
				RiskScore: 60,
			},
			want: true,
		},
		{
			name: "high risk CREATE INDEX without CONCURRENTLY returns true",
			op: &models.Operation{
				SQL:       "CREATE INDEX idx_email ON users(email)",
				Type:      models.OperationTypeCreateIndex,
				RiskScore: 70,
			},
			want: true,
		},
		{
			name: "high risk CREATE INDEX with CONCURRENTLY returns false",
			op: &models.Operation{
				SQL:       "CREATE INDEX CONCURRENTLY idx_email ON users(email)",
				Type:      models.OperationTypeCreateIndex,
				RiskScore: 70,
			},
			want: false,
		},
		{
			name: "high risk ALTER COLUMN TYPE returns true",
			op: &models.Operation{
				SQL:       "ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(500)",
				Type:      models.OperationTypeAlterColumn,
				RiskScore: 80,
			},
			want: true,
		},
		{
			name: "high risk unsupported operation returns false",
			op: &models.Operation{
				SQL:       "DROP TABLE users",
				Type:      models.OperationTypeDropTable,
				RiskScore: 90,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gen.CanGenerateAlternative(tt.op)
			if got != tt.want {
				t.Errorf("CanGenerateAlternative() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPostgresGenerator_AddColumnWithDefault(t *testing.T) {
	gen, err := GetAlternativeGenerator("postgresql")
	if err != nil {
		t.Fatalf("Failed to get generator: %v", err)
	}

	op := &models.Operation{
		SQL:                  "ALTER TABLE users ADD COLUMN age INT DEFAULT 0",
		Type:                 models.OperationTypeAddColumn,
		TableName:            "users",
		RiskScore:            65,
		EstimatedTimeSeconds: 120.0,
	}

	ctx := context.Background()
	alternatives, err := gen.GenerateAlternatives(ctx, op)
	if err != nil {
		t.Fatalf("GenerateAlternatives() error = %v", err)
	}

	if len(alternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(alternatives))
	}

	alt := alternatives[0]

	// Verify strategy metadata
	if alt.StrategyName != "Multi-step Column Addition" {
		t.Errorf("StrategyName = %q, want %q", alt.StrategyName, "Multi-step Column Addition")
	}

	if alt.RiskReduction != 50 {
		t.Errorf("RiskReduction = %d, want 50", alt.RiskReduction)
	}

	// Verify tradeoffs
	if len(alt.Tradeoffs) == 0 {
		t.Error("expected tradeoffs, got none")
	}
	expectedTradeoffs := []string{
		"Requires multiple deployments",
		"Column will be NULL initially",
		"Need to handle NULL in application code temporarily",
	}
	if len(alt.Tradeoffs) != len(expectedTradeoffs) {
		t.Errorf("expected %d tradeoffs, got %d", len(expectedTradeoffs), len(alt.Tradeoffs))
	}

	// Verify 3 steps
	if len(alt.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(alt.Steps))
	}

	// Step 1: ADD COLUMN without DEFAULT
	step1 := alt.Steps[0]
	if step1.StepNumber != 1 {
		t.Errorf("Step 1 StepNumber = %d, want 1", step1.StepNumber)
	}
	if step1.Phase != models.PhasePreDeploy {
		t.Errorf("Step 1 Phase = %q, want %q", step1.Phase, models.PhasePreDeploy)
	}
	if !strings.Contains(step1.SQL, "ADD COLUMN") {
		t.Errorf("Step 1 SQL should contain ADD COLUMN, got: %s", step1.SQL)
	}
	if strings.Contains(step1.SQL, "DEFAULT") {
		t.Errorf("Step 1 SQL should not contain DEFAULT, got: %s", step1.SQL)
	}
	if step1.RiskScore != 15 {
		t.Errorf("Step 1 RiskScore = %d, want 15", step1.RiskScore)
	}
	if step1.EstimatedTime != 0.1 {
		t.Errorf("Step 1 EstimatedTime = %f, want 0.1", step1.EstimatedTime)
	}

	// Step 2: UPDATE to backfill
	step2 := alt.Steps[1]
	if step2.StepNumber != 2 {
		t.Errorf("Step 2 StepNumber = %d, want 2", step2.StepNumber)
	}
	if step2.Phase != models.PhaseBackground {
		t.Errorf("Step 2 Phase = %q, want %q", step2.Phase, models.PhaseBackground)
	}
	if !strings.Contains(step2.SQL, "UPDATE") {
		t.Errorf("Step 2 SQL should contain UPDATE, got: %s", step2.SQL)
	}
	if step2.RiskScore != 20 {
		t.Errorf("Step 2 RiskScore = %d, want 20", step2.RiskScore)
	}
	// Should be 50% of original time (120 * 0.5 = 60)
	if step2.EstimatedTime != 60.0 {
		t.Errorf("Step 2 EstimatedTime = %f, want 60.0", step2.EstimatedTime)
	}
	if !step2.CanRunOffline {
		t.Errorf("Step 2 CanRunOffline = false, want true")
	}

	// Step 3: ALTER COLUMN SET DEFAULT
	step3 := alt.Steps[2]
	if step3.StepNumber != 3 {
		t.Errorf("Step 3 StepNumber = %d, want 3", step3.StepNumber)
	}
	if step3.Phase != models.PhaseDuringDeploy {
		t.Errorf("Step 3 Phase = %q, want %q", step3.Phase, models.PhaseDuringDeploy)
	}
	if !strings.Contains(step3.SQL, "SET DEFAULT") {
		t.Errorf("Step 3 SQL should contain SET DEFAULT, got: %s", step3.SQL)
	}
	if step3.RiskScore != 10 {
		t.Errorf("Step 3 RiskScore = %d, want 10", step3.RiskScore)
	}
	if step3.EstimatedTime != 0.1 {
		t.Errorf("Step 3 EstimatedTime = %f, want 0.1", step3.EstimatedTime)
	}

	// Verify total estimated time (0.1 + 60 + 0.1 = 60.2)
	expectedTotal := 60.2
	if alt.EstimatedTime != expectedTotal {
		t.Errorf("EstimatedTime = %f, want %f", alt.EstimatedTime, expectedTotal)
	}
}

func TestPostgresGenerator_CreateIndexConcurrently(t *testing.T) {
	gen, err := GetAlternativeGenerator("postgresql")
	if err != nil {
		t.Fatalf("Failed to get generator: %v", err)
	}

	op := &models.Operation{
		SQL:                  "CREATE INDEX idx_email ON users(email)",
		Type:                 models.OperationTypeCreateIndex,
		TableName:            "users",
		RiskScore:            70,
		EstimatedTimeSeconds: 100.0,
	}

	ctx := context.Background()
	alternatives, err := gen.GenerateAlternatives(ctx, op)
	if err != nil {
		t.Fatalf("GenerateAlternatives() error = %v", err)
	}

	if len(alternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(alternatives))
	}

	alt := alternatives[0]

	// Verify strategy metadata
	if alt.StrategyName != "Concurrent Index Creation" {
		t.Errorf("StrategyName = %q, want %q", alt.StrategyName, "Concurrent Index Creation")
	}

	if alt.RiskReduction != 30 {
		t.Errorf("RiskReduction = %d, want 30", alt.RiskReduction)
	}

	// Verify tradeoffs
	expectedTradeoffs := []string{
		"Takes longer than regular index creation",
		"Cannot run inside a transaction",
		"May fail and leave invalid index that needs cleanup",
	}
	if len(alt.Tradeoffs) != len(expectedTradeoffs) {
		t.Errorf("expected %d tradeoffs, got %d", len(expectedTradeoffs), len(alt.Tradeoffs))
	}

	// Verify 1 step
	if len(alt.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(alt.Steps))
	}

	// Step 1: CREATE INDEX CONCURRENTLY
	step := alt.Steps[0]
	if step.StepNumber != 1 {
		t.Errorf("StepNumber = %d, want 1", step.StepNumber)
	}
	if step.Phase != models.PhaseDuringDeploy {
		t.Errorf("Phase = %q, want %q", step.Phase, models.PhaseDuringDeploy)
	}
	if !strings.Contains(step.SQL, "CONCURRENTLY") {
		t.Errorf("SQL should contain CONCURRENTLY, got: %s", step.SQL)
	}
	if step.RiskScore != 20 {
		t.Errorf("RiskScore = %d, want 20", step.RiskScore)
	}
	// Should be 150% of original time (100 * 1.5 = 150)
	if step.EstimatedTime != 150.0 {
		t.Errorf("EstimatedTime = %f, want 150.0", step.EstimatedTime)
	}

	// Verify total estimated time
	if alt.EstimatedTime != 150.0 {
		t.Errorf("EstimatedTime = %f, want 150.0", alt.EstimatedTime)
	}
}

func TestPostgresGenerator_AlterColumnType(t *testing.T) {
	gen, err := GetAlternativeGenerator("postgresql")
	if err != nil {
		t.Fatalf("Failed to get generator: %v", err)
	}

	op := &models.Operation{
		SQL:                  "ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(500)",
		Type:                 models.OperationTypeAlterColumn,
		TableName:            "users",
		RiskScore:            80,
		EstimatedTimeSeconds: 200.0,
	}

	ctx := context.Background()
	alternatives, err := gen.GenerateAlternatives(ctx, op)
	if err != nil {
		t.Fatalf("GenerateAlternatives() error = %v", err)
	}

	if len(alternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(alternatives))
	}

	alt := alternatives[0]

	// Verify strategy metadata
	if alt.StrategyName != "Multi-column Type Change" {
		t.Errorf("StrategyName = %q, want %q", alt.StrategyName, "Multi-column Type Change")
	}

	if alt.Description != "Add new column with desired type, copy data, rename" {
		t.Errorf("Description = %q, want %q", alt.Description, "Add new column with desired type, copy data, rename")
	}

	if alt.RiskReduction != 40 {
		t.Errorf("RiskReduction = %d, want 40", alt.RiskReduction)
	}

	// Verify tradeoffs
	expectedTradeoffs := []string{
		"Requires multiple deployments",
		"Temporary storage increase (both columns exist)",
		"Application must be updated to use new column",
	}
	if len(alt.Tradeoffs) != len(expectedTradeoffs) {
		t.Errorf("expected %d tradeoffs, got %d", len(expectedTradeoffs), len(alt.Tradeoffs))
	}

	// Verify at least 1 step exists
	if len(alt.Steps) == 0 {
		t.Fatal("expected at least 1 step, got 0")
	}
}

func TestPostgresGenerator_NoAlternativesForLowRisk(t *testing.T) {
	gen, err := GetAlternativeGenerator("postgresql")
	if err != nil {
		t.Fatalf("Failed to get generator: %v", err)
	}

	op := &models.Operation{
		SQL:       "ALTER TABLE users ADD COLUMN name VARCHAR(100)",
		Type:      models.OperationTypeAddColumn,
		RiskScore: 30, // Low risk
	}

	ctx := context.Background()
	alternatives, err := gen.GenerateAlternatives(ctx, op)
	if err != nil {
		t.Fatalf("GenerateAlternatives() error = %v", err)
	}

	// Should return empty slice, not error
	if alternatives == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(alternatives) != 0 {
		t.Errorf("expected 0 alternatives for low risk operation, got %d", len(alternatives))
	}
}

func TestPostgresGenerator_NoAlternativesForUnsupportedType(t *testing.T) {
	gen, err := GetAlternativeGenerator("postgresql")
	if err != nil {
		t.Fatalf("Failed to get generator: %v", err)
	}

	op := &models.Operation{
		SQL:       "DROP TABLE users",
		Type:      models.OperationTypeDropTable,
		RiskScore: 90, // High risk
	}

	ctx := context.Background()
	alternatives, err := gen.GenerateAlternatives(ctx, op)
	if err != nil {
		t.Fatalf("GenerateAlternatives() error = %v", err)
	}

	// Should return empty slice, not error
	if alternatives == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(alternatives) != 0 {
		t.Errorf("expected 0 alternatives for unsupported operation type, got %d", len(alternatives))
	}
}

func TestPostgresGenerator_HelperMethods(t *testing.T) {
	t.Run("hasDefaultValue", func(t *testing.T) {
		tests := []struct {
			sql  string
			want bool
		}{
			{"ALTER TABLE users ADD COLUMN age INT DEFAULT 0", true},
			{"ALTER TABLE users ADD COLUMN age INT default 0", true},
			{"ALTER TABLE users ADD COLUMN age INT", false},
			{"ALTER TABLE users ADD COLUMN def INT", false}, // "def" in column name
		}

		gen := &postgresGenerator{}
		for _, tt := range tests {
			got := gen.hasDefaultValue(tt.sql)
			if got != tt.want {
				t.Errorf("hasDefaultValue(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		}
	})

	t.Run("hasConcurrently", func(t *testing.T) {
		tests := []struct {
			sql  string
			want bool
		}{
			{"CREATE INDEX CONCURRENTLY idx_email ON users(email)", true},
			{"CREATE INDEX concurrently idx_email ON users(email)", true},
			{"CREATE INDEX idx_email ON users(email)", false},
		}

		gen := &postgresGenerator{}
		for _, tt := range tests {
			got := gen.hasConcurrently(tt.sql)
			if got != tt.want {
				t.Errorf("hasConcurrently(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		}
	})

	t.Run("isTypeChange", func(t *testing.T) {
		tests := []struct {
			sql  string
			want bool
		}{
			{"ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(500)", true},
			{"ALTER TABLE users ALTER COLUMN email type VARCHAR(500)", true},
			{"ALTER TABLE users ALTER COLUMN email SET NOT NULL", false},
			{"ALTER TABLE users ADD COLUMN email VARCHAR(255)", false},
		}

		gen := &postgresGenerator{}
		for _, tt := range tests {
			got := gen.isTypeChange(tt.sql)
			if got != tt.want {
				t.Errorf("isTypeChange(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		}
	})
}

func TestPostgresGenerator_EdgeCases(t *testing.T) {
	gen := &postgresGenerator{}

	t.Run("CONCURRENTLY case insensitivity", func(t *testing.T) {
		sql := "create index idx on users(id)"
		alt := gen.generateConcurrentIndexAlternative(&models.Operation{
			SQL:                  sql,
			Type:                 models.OperationTypeCreateIndex,
			TableName:            "users",
			RiskScore:            70,
			EstimatedTimeSeconds: 100.0,
		})
		if !strings.Contains(strings.ToUpper(alt.Steps[0].SQL), "CONCURRENTLY") {
			t.Errorf("Expected CONCURRENTLY in SQL, got: %s", alt.Steps[0].SQL)
		}
	})

	t.Run("complex type with comma", func(t *testing.T) {
		sql := "ALTER TABLE users ADD COLUMN price NUMERIC(10,2) DEFAULT 0.00"
		colType := gen.extractColumnType(sql)
		if colType != "NUMERIC(10,2)" {
			t.Errorf("extractColumnType() = %q, want %q", colType, "NUMERIC(10,2)")
		}
	})

	t.Run("default value with spaces", func(t *testing.T) {
		sql := "ALTER TABLE users ADD COLUMN name VARCHAR(100) DEFAULT 'John Doe'"
		defValue := gen.extractDefaultValue(sql)
		if defValue != "'John Doe'" {
			t.Errorf("extractDefaultValue() = %q, want %q", defValue, "'John Doe'")
		}
	})

	t.Run("quoted column name", func(t *testing.T) {
		sql := `ALTER TABLE users ADD COLUMN "user name" VARCHAR(100)`
		colName := gen.extractColumnName(sql)
		if colName != "user name" {
			t.Errorf("extractColumnName() = %q, want %q", colName, "user name")
		}
	})
}
