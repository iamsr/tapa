package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/dma/pkg/models"
)

func TestLoad_FromFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".dma.yml")

	configContent := `
database:
  type: postgresql
  host: localhost
  port: 5432
  name: testdb
  user: testuser
  password: testpass

analysis:
  disk_throughput_mbps: 300
  rewrite_factor: 2.5

output:
  format: json
  fail_on_risk_level: HIGH
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Assert database config
	assert.Equal(t, "postgresql", cfg.Database.Type)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "testuser", cfg.Database.User)
	assert.Equal(t, "testpass", cfg.Database.Password)

	// Assert analysis config
	assert.Equal(t, 300, cfg.Analysis.DiskThroughputMBps)
	assert.Equal(t, 2.5, cfg.Analysis.RewriteFactor)

	// Assert output config
	assert.Equal(t, "json", cfg.Output.Format)
	assert.Equal(t, models.RiskLevelHigh, cfg.Output.FailOnRiskLevel)
}

func TestLoad_Defaults(t *testing.T) {
	// Load with empty path should return defaults
	cfg, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Assert default database config
	assert.Equal(t, "postgresql", cfg.Database.Type)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "", cfg.Database.Name)
	assert.Equal(t, "", cfg.Database.User)
	assert.Equal(t, "", cfg.Database.Password)

	// Assert default analysis config
	assert.Equal(t, 200, cfg.Analysis.DiskThroughputMBps)
	assert.Equal(t, 2.0, cfg.Analysis.RewriteFactor)

	// Assert default output config
	assert.Equal(t, "table", cfg.Output.Format)
	assert.Equal(t, models.RiskLevel(""), cfg.Output.FailOnRiskLevel)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid postgresql config",
			config: &Config{
				Database: DatabaseConfig{
					Type: "postgresql",
					Host: "localhost",
					Port: 5432,
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      2.0,
				},
				Output: OutputConfig{
					Format:          "table",
					FailOnRiskLevel: models.RiskLevelHigh,
				},
			},
			wantErr: false,
		},
		{
			name: "valid mysql config",
			config: &Config{
				Database: DatabaseConfig{
					Type: "mysql",
					Host: "localhost",
					Port: 3306,
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      2.0,
				},
				Output: OutputConfig{
					Format: "json",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid database type",
			config: &Config{
				Database: DatabaseConfig{
					Type: "invalid",
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      2.0,
				},
				Output: OutputConfig{
					Format: "table",
				},
			},
			wantErr: true,
			errMsg:  "unsupported database type",
		},
		{
			name: "invalid output format",
			config: &Config{
				Database: DatabaseConfig{
					Type: "postgresql",
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      2.0,
				},
				Output: OutputConfig{
					Format: "xml",
				},
			},
			wantErr: true,
			errMsg:  "unsupported output format",
		},
		{
			name: "invalid risk level",
			config: &Config{
				Database: DatabaseConfig{
					Type: "postgresql",
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      2.0,
				},
				Output: OutputConfig{
					Format:          "table",
					FailOnRiskLevel: models.RiskLevel("INVALID"),
				},
			},
			wantErr: true,
			errMsg:  "unsupported risk level",
		},
		{
			name: "negative disk throughput",
			config: &Config{
				Database: DatabaseConfig{
					Type: "postgresql",
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: -100,
					RewriteFactor:      2.0,
				},
				Output: OutputConfig{
					Format: "table",
				},
			},
			wantErr: true,
			errMsg:  "disk_throughput_mbps must be positive",
		},
		{
			name: "negative rewrite factor",
			config: &Config{
				Database: DatabaseConfig{
					Type: "postgresql",
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      -1.0,
				},
				Output: OutputConfig{
					Format: "table",
				},
			},
			wantErr: true,
			errMsg:  "rewrite_factor must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
