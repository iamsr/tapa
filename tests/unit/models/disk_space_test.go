package models_test

import (
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestDiskSpaceAnalysis_String(t *testing.T) {
	analysis := &models.DiskSpaceAnalysis{
		CurrentState: models.DiskSpaceState{
			TableSizeBytes: 80 * 1024 * 1024 * 1024, // 80GB
			IndexSizeBytes: 15 * 1024 * 1024 * 1024, // 15GB
			TotalSizeBytes: 95 * 1024 * 1024 * 1024, // 95GB
		},
		MigrationRequirements: models.MigrationDiskRequirements{
			RequiresRewrite:     true,
			TemporaryTableBytes: 85 * 1024 * 1024 * 1024,  // 85GB
			PeakDiskUsageBytes:  196 * 1024 * 1024 * 1024, // 196GB
		},
		SystemCheck: models.SystemDiskCheck{
			AvailableBytes:     150 * 1024 * 1024 * 1024, // 150GB
			RequiredBytes:      196 * 1024 * 1024 * 1024, // 196GB
			HasSufficientSpace: false,
			ShortfallBytes:     46 * 1024 * 1024 * 1024, // 46GB
		},
	}

	// Should not panic
	_ = analysis.String()
}

func TestDiskSpaceAnalysis_HasSufficientSpace(t *testing.T) {
	tests := []struct {
		name           string
		availableBytes int64
		requiredBytes  int64
		want           bool
	}{
		{"sufficient", 200 * GB, 150 * GB, true},
		{"insufficient", 150 * GB, 200 * GB, false},
		{"exact", 100 * GB, 100 * GB, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &models.DiskSpaceAnalysis{
				SystemCheck: models.SystemDiskCheck{
					AvailableBytes: tt.availableBytes,
					RequiredBytes:  tt.requiredBytes,
				},
			}
			analysis.SystemCheck.HasSufficientSpace = analysis.SystemCheck.AvailableBytes >= analysis.SystemCheck.RequiredBytes

			if got := analysis.SystemCheck.HasSufficientSpace; got != tt.want {
				t.Errorf("HasSufficientSpace = %v, want %v", got, tt.want)
			}
		})
	}
}

const GB = 1024 * 1024 * 1024
