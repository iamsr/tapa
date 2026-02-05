package models

import (
	"testing"
)

func TestAlternativeStrategy_TotalRiskReduction(t *testing.T) {
	strategy := AlternativeStrategy{
		StrategyName: "Multi-step Column Add",
		Description:  "Add column without default, then backfill, then add default",
		Steps: []AlternativeStep{
			{
				StepNumber:        1,
				Phase:             PhasePreDeploy,
				SQL:               "ALTER TABLE users ADD COLUMN email TEXT NULL;",
				Description:       "Add nullable column",
				RequiresAppChange: false,
				RiskScore:         10,
				EstimatedTime:     0.5,
				CanRunOffline:     false,
			},
			{
				StepNumber:        2,
				Phase:             PhaseBackground,
				SQL:               "UPDATE users SET email = generate_email(id);",
				Description:       "Backfill email values",
				RequiresAppChange: false,
				RiskScore:         5,
				EstimatedTime:     120.0,
				CanRunOffline:     true,
			},
			{
				StepNumber:        3,
				Phase:             PhasePostDeploy,
				SQL:               "ALTER TABLE users ALTER COLUMN email SET NOT NULL;",
				Description:       "Add NOT NULL constraint",
				RequiresAppChange: true,
				RiskScore:         15,
				EstimatedTime:     0.3,
				CanRunOffline:     false,
			},
		},
		RiskReduction: 50,
		Tradeoffs:     []string{"Requires multiple deployments", "Takes longer overall"},
		EstimatedTime: 120.8,
	}

	// Test that strategy has multiple steps
	if len(strategy.Steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(strategy.Steps))
	}

	// Test that risk reduction is properly set
	if strategy.RiskReduction != 50 {
		t.Errorf("Expected risk reduction of 50, got %d", strategy.RiskReduction)
	}

	// Test that each step has reduced risk compared to original operation (assumed 60)
	for i, step := range strategy.Steps {
		if step.RiskScore >= 60 {
			t.Errorf("Step %d has risk score %d, which is not reduced from original", i+1, step.RiskScore)
		}
	}

	// Test that total estimated time is calculated
	if strategy.EstimatedTime <= 0 {
		t.Errorf("Expected positive estimated time, got %.2f", strategy.EstimatedTime)
	}
}

func TestAlternativeStep_PhaseValidation(t *testing.T) {
	tests := []struct {
		name          string
		step          AlternativeStep
		expectedPhase Phase
	}{
		{
			name: "Pre-deploy phase",
			step: AlternativeStep{
				StepNumber:  1,
				Phase:       PhasePreDeploy,
				SQL:         "CREATE INDEX CONCURRENTLY idx_users_email ON users(email);",
				Description: "Create index before code deployment",
			},
			expectedPhase: PhasePreDeploy,
		},
		{
			name: "During deploy phase",
			step: AlternativeStep{
				StepNumber:  2,
				Phase:       PhaseDuringDeploy,
				SQL:         "ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active';",
				Description: "Add column during deployment",
			},
			expectedPhase: PhaseDuringDeploy,
		},
		{
			name: "Post-deploy phase",
			step: AlternativeStep{
				StepNumber:  3,
				Phase:       PhasePostDeploy,
				SQL:         "ALTER TABLE users ALTER COLUMN status SET NOT NULL;",
				Description: "Add constraint after code is deployed",
			},
			expectedPhase: PhasePostDeploy,
		},
		{
			name: "Background phase",
			step: AlternativeStep{
				StepNumber:    4,
				Phase:         PhaseBackground,
				SQL:           "UPDATE users SET status = 'active' WHERE status IS NULL;",
				Description:   "Backfill data in background",
				CanRunOffline: true,
			},
			expectedPhase: PhaseBackground,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.step.Phase != tt.expectedPhase {
				t.Errorf("Expected phase %s, got %s", tt.expectedPhase, tt.step.Phase)
			}
		})
	}
}

func TestAlternativeStep_RequiresAppChange(t *testing.T) {
	step := AlternativeStep{
		StepNumber:        1,
		Phase:             PhasePostDeploy,
		SQL:               "ALTER TABLE users ALTER COLUMN email SET NOT NULL;",
		Description:       "Add NOT NULL constraint",
		RequiresAppChange: true,
		RiskScore:         15,
	}

	if !step.RequiresAppChange {
		t.Error("Expected step to require app change")
	}
}

func TestAlternativeStep_CanRunOffline(t *testing.T) {
	step := AlternativeStep{
		StepNumber:    1,
		Phase:         PhaseBackground,
		SQL:           "UPDATE users SET email = generate_email(id) WHERE email IS NULL;",
		Description:   "Backfill email values",
		CanRunOffline: true,
		EstimatedTime: 300.0,
	}

	if !step.CanRunOffline {
		t.Error("Expected step to be able to run offline")
	}

	if step.EstimatedTime != 300.0 {
		t.Errorf("Expected estimated time of 300.0, got %.2f", step.EstimatedTime)
	}
}

func TestAlternativeStrategy_EmptySteps(t *testing.T) {
	strategy := AlternativeStrategy{
		StrategyName:  "Empty Strategy",
		Description:   "Strategy with no steps",
		Steps:         []AlternativeStep{},
		RiskReduction: 0,
		Tradeoffs:     []string{},
		EstimatedTime: 0,
	}

	if len(strategy.Steps) != 0 {
		t.Errorf("Expected 0 steps, got %d", len(strategy.Steps))
	}

	if strategy.EstimatedTime != 0 {
		t.Errorf("Expected 0 estimated time, got %.2f", strategy.EstimatedTime)
	}
}

func TestAlternativeStrategy_Tradeoffs(t *testing.T) {
	strategy := AlternativeStrategy{
		StrategyName: "Gradual Migration",
		Description:  "Migrate data gradually",
		Steps: []AlternativeStep{
			{StepNumber: 1, Phase: PhasePreDeploy, SQL: "CREATE TABLE users_new (...);"},
			{StepNumber: 2, Phase: PhaseBackground, SQL: "INSERT INTO users_new SELECT * FROM users;"},
			{StepNumber: 3, Phase: PhasePostDeploy, SQL: "DROP TABLE users;"},
		},
		RiskReduction: 40,
		Tradeoffs: []string{
			"Requires application to write to both tables",
			"More complex rollback procedure",
			"Increased storage during migration",
		},
		EstimatedTime: 500.0,
	}

	if len(strategy.Tradeoffs) != 3 {
		t.Errorf("Expected 3 tradeoffs, got %d", len(strategy.Tradeoffs))
	}

	expectedTradeoff := "Requires application to write to both tables"
	if strategy.Tradeoffs[0] != expectedTradeoff {
		t.Errorf("Expected first tradeoff to be %q, got %q", expectedTradeoff, strategy.Tradeoffs[0])
	}
}

func TestPhaseConstants(t *testing.T) {
	// Test that phase constants are correctly defined
	phases := []Phase{
		PhasePreDeploy,
		PhaseDuringDeploy,
		PhasePostDeploy,
		PhaseBackground,
	}

	// Test that phases are distinct
	seen := make(map[Phase]bool)
	for _, phase := range phases {
		if seen[phase] {
			t.Errorf("Duplicate phase constant: %s", phase)
		}
		seen[phase] = true
	}

	// Test that we have exactly 4 phases
	if len(phases) != 4 {
		t.Errorf("Expected 4 phase constants, got %d", len(phases))
	}
}
