package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iamsr/tapa/internal/output"
	"github.com/iamsr/tapa/pkg/models"
)

func TestFormatAdvancedFeatures(t *testing.T) {
	const GB = 1024 * 1024 * 1024

	op := &models.Operation{
		Type:      models.OperationTypeAlterColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ALTER COLUMN email TYPE TEXT",
		DiskSpaceAnalysis: &models.DiskSpaceAnalysis{
			CurrentState: models.DiskSpaceState{
				TableSizeBytes: 85 * GB,
				IndexSizeBytes: 10 * GB,
				TotalSizeBytes: 95 * GB,
			},
			MigrationRequirements: models.MigrationDiskRequirements{
				RequiresRewrite:     true,
				TemporaryTableBytes: 90 * GB,
				PeakDiskUsageBytes:  196 * GB,
			},
			SystemCheck: models.SystemDiskCheck{
				AvailableBytes:     150 * GB,
				RequiredBytes:      196 * GB,
				HasSufficientSpace: false,
				ShortfallBytes:     46 * GB,
			},
		},
		RollbackAnalysis: &models.RollbackAnalysis{
			Category:           models.ReversibilityDataLoss,
			ReversibilityScore: 25,
			IsReversible:       false,
			Reason:             "Type conversion may cause precision loss",
		},
	}

	var buf bytes.Buffer
	err := output.FormatAdvancedFeatures(&buf, op)
	if err != nil {
		t.Fatalf("FormatAdvancedFeatures failed: %v", err)
	}

	output := buf.String()

	// Verify output contains key information
	if !strings.Contains(output, "Disk Space Analysis") {
		t.Error("Output should contain 'Disk Space Analysis' section")
	}

	if !strings.Contains(output, "Rollback Analysis") {
		t.Error("Output should contain 'Rollback Analysis' section")
	}

	if !strings.Contains(output, "INSUFFICIENT") {
		t.Error("Output should indicate insufficient disk space")
	}

	// Print sample output for review
	if testing.Verbose() {
		t.Log("Sample output:")
		t.Log(output)
	}
}
