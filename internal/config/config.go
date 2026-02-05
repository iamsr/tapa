package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Analysis AnalysisConfig `mapstructure:"analysis"`
	Output   OutputConfig   `mapstructure:"output"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	URL  string `mapstructure:"url"`
	Type string `mapstructure:"type"`
}

// AnalysisConfig holds analysis parameters
type AnalysisConfig struct {
	DiskThroughputMBps int     `mapstructure:"disk_throughput_mbps"`
	RewriteFactor      float64 `mapstructure:"rewrite_factor"`
	FailOnRiskLevel    string  `mapstructure:"fail_on_risk_level"`
}

// OutputConfig holds output formatting settings
type OutputConfig struct {
	Format  string `mapstructure:"format"`
	Verbose bool   `mapstructure:"verbose"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Type: "",
			URL:  "",
		},
		Analysis: AnalysisConfig{
			DiskThroughputMBps: 200, // Conservative SSD throughput
			RewriteFactor:      2.0, // Conservative rewrite estimate
			FailOnRiskLevel:    "",
		},
		Output: OutputConfig{
			Format:  "table",
			Verbose: false,
		},
	}
}

// Load reads configuration from the specified path or returns defaults
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// If no config path provided, return defaults
	if configPath == "" {
		return cfg, nil
	}

	// Configure viper
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into config struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate database type
	validDatabaseTypes := []string{"postgresql", "mysql"}
	if !contains(validDatabaseTypes, c.Database.Type) {
		return fmt.Errorf("unsupported database type '%s', must be one of: %s",
			c.Database.Type, strings.Join(validDatabaseTypes, ", "))
	}

	// Validate output format
	validFormats := []string{"table", "json", "yaml"}
	if !contains(validFormats, c.Output.Format) {
		return fmt.Errorf("unsupported output format '%s', must be one of: %s",
			c.Output.Format, strings.Join(validFormats, ", "))
	}

	// Validate risk level if provided
	if c.Analysis.FailOnRiskLevel != "" {
		validRiskLevels := []string{"low", "medium", "high", "critical"}
		if !contains(validRiskLevels, c.Analysis.FailOnRiskLevel) {
			return fmt.Errorf("unsupported risk level '%s', must be one of: low, medium, high, critical",
				c.Analysis.FailOnRiskLevel)
		}
	}

	// Validate analysis parameters
	if c.Analysis.DiskThroughputMBps <= 0 {
		return fmt.Errorf("disk_throughput_mbps must be positive, got %d", c.Analysis.DiskThroughputMBps)
	}

	if c.Analysis.RewriteFactor <= 0 {
		return fmt.Errorf("rewrite_factor must be positive, got %.2f", c.Analysis.RewriteFactor)
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
