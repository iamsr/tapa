package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/yourusername/dma/pkg/models"
)

// Config represents the application configuration
type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Analysis AnalysisConfig `mapstructure:"analysis"`
	Output   OutputConfig   `mapstructure:"output"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Type     string `mapstructure:"type"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

// AnalysisConfig holds analysis parameters
type AnalysisConfig struct {
	DiskThroughputMBps int     `mapstructure:"disk_throughput_mbps"`
	RewriteFactor      float64 `mapstructure:"rewrite_factor"`
}

// OutputConfig holds output formatting settings
type OutputConfig struct {
	Format          string           `mapstructure:"format"`
	FailOnRiskLevel models.RiskLevel `mapstructure:"fail_on_risk_level"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Type: "postgresql",
			Host: "localhost",
			Port: 5432,
		},
		Analysis: AnalysisConfig{
			DiskThroughputMBps: 200, // Conservative SSD throughput
			RewriteFactor:      2.0, // Conservative rewrite estimate
		},
		Output: OutputConfig{
			Format: "table",
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
	if c.Output.FailOnRiskLevel != "" {
		validRiskLevels := []models.RiskLevel{
			models.RiskLevelLow,
			models.RiskLevelMedium,
			models.RiskLevelHigh,
			models.RiskLevelCritical,
		}
		if !containsRiskLevel(validRiskLevels, c.Output.FailOnRiskLevel) {
			return fmt.Errorf("unsupported risk level '%s', must be one of: LOW, MEDIUM, HIGH, CRITICAL",
				c.Output.FailOnRiskLevel)
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

// containsRiskLevel checks if a slice contains a risk level
func containsRiskLevel(slice []models.RiskLevel, item models.RiskLevel) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
