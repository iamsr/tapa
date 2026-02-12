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
