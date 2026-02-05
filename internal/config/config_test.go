package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".dma.yml")

	configContent := `
database:
  url: postgres://testuser:testpass@localhost:5432/testdb
  type: postgresql

analysis:
  disk_throughput_mbps: 300
  rewrite_factor: 2.5
  fail_on_risk_level: high

output:
  format: json
  verbose: true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Assert database config
	assert.Equal(t, "postgres://testuser:testpass@localhost:5432/testdb", cfg.Database.URL)
	assert.Equal(t, "postgresql", cfg.Database.Type)

	// Assert analysis config
	assert.Equal(t, 300, cfg.Analysis.DiskThroughputMBps)
	assert.Equal(t, 2.5, cfg.Analysis.RewriteFactor)
	assert.Equal(t, "high", cfg.Analysis.FailOnRiskLevel)

	// Assert output config
	assert.Equal(t, "json", cfg.Output.Format)
	assert.Equal(t, true, cfg.Output.Verbose)
}

func TestLoad_Defaults(t *testing.T) {
	// Load with empty path should return defaults
	cfg, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Assert default database config (empty)
	assert.Equal(t, "", cfg.Database.URL)
	assert.Equal(t, "", cfg.Database.Type)

	// Assert default analysis config
	assert.Equal(t, 200, cfg.Analysis.DiskThroughputMBps)
	assert.Equal(t, 2.0, cfg.Analysis.RewriteFactor)
	assert.Equal(t, "", cfg.Analysis.FailOnRiskLevel)

	// Assert default output config
	assert.Equal(t, "table", cfg.Output.Format)
	assert.Equal(t, false, cfg.Output.Verbose)
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
					URL:  "postgres://localhost/testdb",
					Type: "postgresql",
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      2.0,
					FailOnRiskLevel:    "high",
				},
				Output: OutputConfig{
					Format:  "table",
					Verbose: false,
				},
			},
			wantErr: false,
		},
		{
			name: "valid mysql config",
			config: &Config{
				Database: DatabaseConfig{
					URL:  "mysql://localhost:3306/testdb",
					Type: "mysql",
				},
				Analysis: AnalysisConfig{
					DiskThroughputMBps: 200,
					RewriteFactor:      2.0,
				},
				Output: OutputConfig{
					Format:  "json",
					Verbose: true,
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
					FailOnRiskLevel:    "INVALID",
				},
				Output: OutputConfig{
					Format: "table",
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
