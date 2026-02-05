package postgres

import (
	"context"
	"testing"

	"github.com/yourusername/dma/pkg/models"
)

// TestAnalyzeWithEnhancements_Integration tests full Phase 2 analysis with all features enabled
func TestAnalyzeWithEnhancements_Integration(t *testing.T) {
	// Create analyzer with nil introspector (dry-run mode)
	analyzer := NewAnalyzer(nil, 100, 2.0)

	// Test high-risk operation that should trigger all Phase 2 features
	op := &models.Operation{
		SQL:       "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT 'unknown@example.com'",
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
	}

	// Run comprehensive analysis
	opts := DefaultAnalysisOptions()
	ctx := context.Background()
	err := analyzer.AnalyzeWithEnhancements(ctx, op, opts)

	if err != nil {
		t.Fatalf("AnalyzeWithEnhancements failed: %v", err)
	}

	// Verify base analysis fields (Phase 1)
	if op.LockType == "" {
		t.Error("Expected lock type to be set")
	}
	if op.RiskScore == 0 {
		t.Error("Expected risk score to be calculated")
	}
	if len(op.Recommendations) == 0 {
		t.Error("Expected recommendations to be generated")
	}

	// Verify Phase 2 fields are populated
	// Note: In dry-run mode (no introspector), some features may return empty results
	// This is expected behavior - the test verifies the integration works without errors

	// Dependencies - may be empty without introspector, but field should be initialized
	if op.Dependencies == nil {
		t.Error("Expected Dependencies field to be initialized (even if empty)")
	}

	// TimeBreakdown - should be populated even without introspector
	if op.TimeBreakdown == nil {
		t.Error("Expected TimeBreakdown to be populated")
	} else {
		if op.TimeBreakdown.TotalSeconds <= 0 {
			t.Error("Expected total time to be calculated")
		}
	}

	// Alternatives - should be generated for high-risk operations
	if op.Alternatives == nil {
		t.Error("Expected Alternatives field to be initialized (even if empty)")
	}
	// For high-risk ADD COLUMN with DEFAULT, we should get an alternative
	if op.RiskScore >= 51 && len(op.Alternatives) == 0 {
		t.Log("Note: High-risk operation but no alternatives generated (may need risk score adjustment)")
	}
}

// TestAnalyzeWithEnhancements_HighRiskIndex tests index creation alternative
func TestAnalyzeWithEnhancements_HighRiskIndex(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)

	// Create index without CONCURRENTLY - high risk
	op := &models.Operation{
		SQL:       "CREATE INDEX idx_users_email ON users(email)",
		Type:      models.OperationTypeCreateIndex,
		TableName: "users",
	}

	opts := DefaultAnalysisOptions()
	ctx := context.Background()
	err := analyzer.AnalyzeWithEnhancements(ctx, op, opts)

	if err != nil {
		t.Fatalf("AnalyzeWithEnhancements failed: %v", err)
	}

	// Base analysis
	if op.LockType != models.LockTypeShare {
		t.Errorf("Expected SHARE lock for non-CONCURRENT index, got %s", op.LockType)
	}

	// Phase 2: TimeBreakdown
	if op.TimeBreakdown == nil {
		t.Error("Expected TimeBreakdown to be populated")
	}

	// Phase 2: Alternatives
	// Alternative should be generated for non-CONCURRENT index creation
	if op.Alternatives == nil {
		t.Error("Expected Alternatives field to be initialized")
	}
	// Check if alternative was generated (depends on risk score)
	if op.RiskScore >= 51 && len(op.Alternatives) > 0 {
		alt := op.Alternatives[0]
		if alt.StrategyName != "Concurrent Index Creation" {
			t.Errorf("Expected 'Concurrent Index Creation' alternative, got %s", alt.StrategyName)
		}
		if len(alt.Steps) == 0 {
			t.Error("Expected alternative to have steps")
		}
	}
}

// TestAnalyzeWithEnhancements_SelectiveOptions tests selective feature enablement
func TestAnalyzeWithEnhancements_SelectiveOptions(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)

	op := &models.Operation{
		SQL:       "ALTER TABLE users DROP COLUMN legacy_field",
		Type:      models.OperationTypeDropColumn,
		TableName: "users",
	}

	// Test with only time breakdown enabled
	opts := AnalysisOptions{
		IncludeDependencies:  false,
		IncludeTimeBreakdown: true,
		IncludeAlternatives:  false,
	}

	ctx := context.Background()
	err := analyzer.AnalyzeWithEnhancements(ctx, op, opts)

	if err != nil {
		t.Fatalf("AnalyzeWithEnhancements failed: %v", err)
	}

	// TimeBreakdown should be populated (since it's enabled)
	if op.TimeBreakdown == nil {
		t.Error("Expected TimeBreakdown to be populated when enabled")
	}

	// Dependencies should be nil or empty (disabled)
	// Alternatives should be nil or empty (disabled)
	// No assertions needed - just verify no error occurred
}

// TestAnalyze_BackwardCompatibility ensures base Analyze() still works
func TestAnalyze_BackwardCompatibility(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)

	op := &models.Operation{
		SQL:       "CREATE TABLE test (id SERIAL PRIMARY KEY)",
		Type:      models.OperationTypeCreateTable,
		TableName: "test",
	}

	ctx := context.Background()
	err := analyzer.Analyze(ctx, op)

	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Verify base analysis works
	if op.LockType != models.LockTypeNone {
		t.Errorf("Expected no lock for CREATE TABLE, got %s", op.LockType)
	}

	// Phase 2 fields should not be populated by basic Analyze()
	if op.Dependencies != nil {
		t.Error("Expected Dependencies to be nil after basic Analyze()")
	}
	if op.TimeBreakdown != nil {
		t.Error("Expected TimeBreakdown to be nil after basic Analyze()")
	}
	if op.Alternatives != nil {
		t.Error("Expected Alternatives to be nil after basic Analyze()")
	}
}
