package output_test

import (
	"bytes"
	"testing"

	"github.com/iamsr/tapa/internal/output"
	"github.com/iamsr/tapa/pkg/models"
)

func TestFormatTable_WithAdvancedFeatures(t *testing.T) {
	const GB = 1024 * 1024 * 1024

	result := &models.AnalysisResult{
		Migrations: []*models.Migration{
			{
				FilePath: "20240212_alter_users_table.sql",
				Operations: []*models.Operation{
					{
						Type:                 models.OperationTypeAlterColumn,
						TableName:            "users",
						ColumnName:           "email",
						SQL:                  "ALTER TABLE users ALTER COLUMN email TYPE TEXT",
						LockType:             models.LockTypeAccessExclusive,
						LockDurationMS:       125000,
						RequiresRewrite:      true,
						EstimatedTimeSeconds: 125.5,
						RiskScore:            65,
						BackwardCompatible:   false,
						RowCount:             10000000,
						TableSizeBytes:       95 * GB,
						Recommendations: []string{
							"Consider using a multi-step approach",
							"Run during maintenance window",
						},
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
							RecoveryStrategy: &models.RecoveryStrategy{
								Method:       "backup_restore",
								EstimatedRTO: "30-60 minutes",
								Steps: []string{
									"Stop application traffic",
									"Restore from backup",
									"Verify data integrity",
								},
							},
						},
						DataMigrationAnalysis: &models.DataMigrationAnalysis{
							HasDataMigration: true,
							OperationType:    "UPDATE",
							Complexity:       models.DataMigrationModerate,
							EstimatedRows:    10000000,
							PerformanceEstimate: &models.PerformanceEstimate{
								BaseSpeedRowsPerSecond:     50000,
								AdjustedSpeedRowsPerSecond: 30000,
								EstimatedDurationSeconds:   333.33,
								EstimatedDurationRange:     "5-6 minutes",
							},
							BatchingRecommendation: &models.BatchingRecommendation{
								ShouldBatch:          true,
								RecommendedBatchSize: 10000,
								TotalBatches:         1000,
								AllowsCancellation:   true,
							},
							TableBloatImpact: &models.TableBloatImpact{
								EstimatedBloatPercent: 25,
								DeadTupleCount:        2500000,
								SpaceReclaimableBytes: 20 * GB,
								VacuumRequired:        true,
								VacuumRecommendation:  "Run VACUUM FULL after migration",
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := output.FormatTable(&buf, result)
	if err != nil {
		t.Fatalf("FormatTable failed: %v", err)
	}

	// Print the complete output
	if testing.Verbose() {
		t.Log("\n=== Complete Output with Advanced Features ===")
		t.Log(buf.String())
		t.Log("=== End Output ===")
	}
}
