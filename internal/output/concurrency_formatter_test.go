package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestFormatConcurrency(t *testing.T) {
	analysis := &models.ConcurrencyAnalysis{
		ImpactScore: 75,
		LockImpact: &models.LockImpact{
			LockType:              models.LockTypeAccessExclusive,
			EstimatedDurationMS:   45000,
			BlocksReads:           true,
			BlocksWrites:          true,
			BlockedQueryTypes:     []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
			EstimatedBlockedCount: 150,
			WaitTimeRange:         "30-60 seconds",
			LockAcquisitionRisk:   "high",
		},
		WorkloadAnalysis: &models.WorkloadAnalysis{
			ActiveConnections:    25,
			QueriesPerSecond:     50.5,
			TableAccessFrequency: "high",
			PeakLoadPeriod:       true,
		},
		SaferAlternatives: []models.ConcurrentAlternative{
			{
				Description:     "Use CREATE INDEX CONCURRENTLY",
				LockType:        models.LockTypeShareUpdateExclusive,
				ImpactReduction: 70,
			},
		},
		Recommendations: []string{
			"Schedule during maintenance window",
			"Monitor active queries before executing",
		},
		ConcurrencySafe: false,
	}

	var buf bytes.Buffer
	err := FormatConcurrencyAnalysis(&buf, analysis)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis() error = %v", err)
	}

	output := buf.String()

	// Check for main section heading
	if !strings.Contains(output, "Concurrency Impact") {
		t.Errorf("Expected output to contain 'Concurrency Impact' section")
	}

	// Check for impact level HIGH
	if !strings.Contains(output, "HIGH") {
		t.Errorf("Expected output to contain impact level 'HIGH'")
	}

	// Check for impact score
	if !strings.Contains(output, "75") {
		t.Errorf("Expected output to contain impact score '75'")
	}

	// Check for lock type
	if !strings.Contains(output, "ACCESS_EXCLUSIVE") {
		t.Errorf("Expected output to contain lock type 'ACCESS_EXCLUSIVE'")
	}

	// Check for blocked query count
	if !strings.Contains(output, "150") {
		t.Errorf("Expected output to contain blocked query count '150'")
	}
}

func TestFormatConcurrency_NilAnalysis(t *testing.T) {
	var buf bytes.Buffer
	err := FormatConcurrencyAnalysis(&buf, nil)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis() with nil should not error: %v", err)
	}

	output := buf.String()
	if output != "" {
		t.Errorf("Expected empty output for nil analysis, got: %s", output)
	}
}

func TestFormatConcurrency_MinimalData(t *testing.T) {
	analysis := &models.ConcurrencyAnalysis{
		ImpactScore:     15,
		ConcurrencySafe: true,
	}

	var buf bytes.Buffer
	err := FormatConcurrencyAnalysis(&buf, analysis)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis() error = %v", err)
	}

	output := buf.String()

	// Check for MINIMAL impact level (score 15 falls in 0-20 range)
	if !strings.Contains(output, "MINIMAL") {
		t.Errorf("Expected output to contain impact level 'MINIMAL', got: %s", output)
	}

	// Check for concurrency safe indicator
	if !strings.Contains(output, "✓ YES") {
		t.Errorf("Expected output to contain '✓ YES' for concurrency safe")
	}

	// Check for impact score
	if !strings.Contains(output, "15") {
		t.Errorf("Expected output to contain impact score '15'")
	}
}

func TestFormatConcurrency_DifferentImpactLevels(t *testing.T) {
	tests := []struct {
		name        string
		impactScore int
		wantLevel   string
	}{
		{"minimal", 10, "MINIMAL"},
		{"low", 30, "LOW"},
		{"medium", 50, "MEDIUM"},
		{"high", 70, "HIGH"},
		{"critical", 90, "CRITICAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.ConcurrencyAnalysis{
				ImpactScore:     tt.impactScore,
				ConcurrencySafe: false,
			}

			var buf bytes.Buffer
			err := FormatConcurrencyAnalysis(&buf, analysis)
			if err != nil {
				t.Fatalf("FormatConcurrencyAnalysis() error = %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, tt.wantLevel) {
				t.Errorf("Expected output to contain impact level '%s' for score %d, got: %s",
					tt.wantLevel, tt.impactScore, output)
			}
		})
	}
}

func TestFormatConcurrency_EmptySlices(t *testing.T) {
	analysis := &models.ConcurrencyAnalysis{
		ImpactScore:       45,
		SaferAlternatives: []models.ConcurrentAlternative{},
		Recommendations:   []string{},
		ConcurrencySafe:   false,
	}

	var buf bytes.Buffer
	err := FormatConcurrencyAnalysis(&buf, analysis)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis() error = %v", err)
	}

	output := buf.String()

	// Should not have Safer Alternatives section if empty
	if strings.Contains(output, "Safer Alternatives:") {
		t.Errorf("Expected no 'Safer Alternatives' section for empty slice")
	}

	// Should not have Recommendations section if empty
	if strings.Contains(output, "Recommendations:") {
		t.Errorf("Expected no 'Recommendations' section for empty slice")
	}
}

func TestFormatConcurrency_LongRunningQueries(t *testing.T) {
	analysis := &models.ConcurrencyAnalysis{
		ImpactScore: 50,
		WorkloadAnalysis: &models.WorkloadAnalysis{
			ActiveConnections:    10,
			QueriesPerSecond:     25.0,
			TableAccessFrequency: "medium",
			PeakLoadPeriod:       false,
			LongRunningQueries:   5,
		},
		ConcurrencySafe: false,
	}

	var buf bytes.Buffer
	err := FormatConcurrencyAnalysis(&buf, analysis)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis() error = %v", err)
	}

	output := buf.String()

	// Check for long-running queries display
	if !strings.Contains(output, "Long-running queries:") {
		t.Errorf("Expected output to contain 'Long-running queries:'")
	}

	if !strings.Contains(output, "5") {
		t.Errorf("Expected output to contain long-running query count '5'")
	}

	if !strings.Contains(output, "may delay lock") {
		t.Errorf("Expected output to contain warning 'may delay lock'")
	}
}

func TestFormatConcurrency_RequiresFeature(t *testing.T) {
	analysis := &models.ConcurrencyAnalysis{
		ImpactScore: 60,
		SaferAlternatives: []models.ConcurrentAlternative{
			{
				Description:     "Use ALGORITHM=INPLACE",
				LockType:        models.LockTypeShare,
				ImpactReduction: 50,
				RequiresFeature: "MySQL 5.7+",
			},
		},
		ConcurrencySafe: false,
	}

	var buf bytes.Buffer
	err := FormatConcurrencyAnalysis(&buf, analysis)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis() error = %v", err)
	}

	output := buf.String()

	// Check for RequiresFeature display
	if !strings.Contains(output, "Requires:") {
		t.Errorf("Expected output to contain 'Requires:'")
	}

	if !strings.Contains(output, "MySQL 5.7+") {
		t.Errorf("Expected output to contain 'MySQL 5.7+'")
	}
}

func TestFormatConcurrency_SaferAlternativesWithStepsAndTradeoffs(t *testing.T) {
	analysis := &models.ConcurrencyAnalysis{
		ImpactScore: 70,
		SaferAlternatives: []models.ConcurrentAlternative{
			{
				Description:     "Use CREATE INDEX CONCURRENTLY",
				LockType:        models.LockTypeShareUpdateExclusive,
				ImpactReduction: 65,
				RequiresFeature: "PostgreSQL 11+",
				Steps: []string{
					"Create index concurrently",
					"Verify index is valid",
				},
				Tradeoffs: []string{
					"Takes longer to complete",
					"Cannot run in transaction",
				},
			},
		},
		ConcurrencySafe: false,
	}

	var buf bytes.Buffer
	err := FormatConcurrencyAnalysis(&buf, analysis)
	if err != nil {
		t.Fatalf("FormatConcurrencyAnalysis() error = %v", err)
	}

	output := buf.String()

	// Check for Steps
	if !strings.Contains(output, "Steps:") {
		t.Errorf("Expected output to contain 'Steps:'")
	}
	if !strings.Contains(output, "Create index concurrently") {
		t.Errorf("Expected output to contain step 'Create index concurrently'")
	}

	// Check for Tradeoffs
	if !strings.Contains(output, "Tradeoffs:") {
		t.Errorf("Expected output to contain 'Tradeoffs:'")
	}
	if !strings.Contains(output, "Takes longer to complete") {
		t.Errorf("Expected output to contain tradeoff 'Takes longer to complete'")
	}

	// Check for RequiresFeature
	if !strings.Contains(output, "PostgreSQL 11+") {
		t.Errorf("Expected output to contain 'PostgreSQL 11+'")
	}
}
