package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/dma/internal/config"
	"github.com/yourusername/dma/internal/output"
	"github.com/yourusername/dma/internal/parser"
	"github.com/yourusername/dma/pkg/models"
)

type analyzeOptions struct {
	dbURL      string
	dbType     string
	format     string
	configFile string
}

func newAnalyzeCommand() *cobra.Command {
	opts := &analyzeOptions{}

	cmd := &cobra.Command{
		Use:   "analyze [migration-file]",
		Short: "Analyze a database migration file",
		Long: `Analyze SQL migration files to predict production impact including:
- DDL operations detected
- Lock types and potential durations
- Risk assessment
- Backward compatibility

The analyzer parses the SQL file and provides detailed insights without
requiring a database connection (though connecting provides enhanced analysis).`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.dbURL, "db-url", "", "database connection URL")
	cmd.Flags().StringVar(&opts.dbType, "db-type", "", "database type (postgresql, mysql)")
	cmd.Flags().StringVar(&opts.format, "format", "table", "output format (table, json, yaml)")

	return cmd
}

func runAnalyze(filePath string, opts *analyzeOptions) error {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	if opts.dbURL != "" {
		cfg.Database.URL = opts.dbURL
	}
	if opts.dbType != "" {
		cfg.Database.Type = opts.dbType
	}
	if opts.format != "" {
		cfg.Output.Format = opts.format
	}

	// Auto-detect database type from URL if not specified
	if cfg.Database.Type == "" && cfg.Database.URL != "" {
		cfg.Database.Type = detectDBType(cfg.Database.URL)
	}

	// If still no database type, default to postgresql
	if cfg.Database.Type == "" {
		cfg.Database.Type = "postgresql"
	}

	// Validate file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("migration file not found: %w", err)
	}

	// Get parser
	sqlParser, err := parser.GetParser(cfg.Database.Type)
	if err != nil {
		return fmt.Errorf("failed to get parser: %w", err)
	}

	// Parse migration file
	migration, err := sqlParser.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse migration: %w", err)
	}

	// Create analysis result
	result := &models.AnalysisResult{
		Migrations:      []*models.Migration{migration},
		DatabaseType:    cfg.Database.Type,
		FailOnRiskLevel: models.RiskLevel(cfg.Analysis.FailOnRiskLevel),
		Errors:          []error{},
	}

	// TODO: Add database introspection in future tasks
	// For now, we're in parse-only mode

	// Output results
	if err := output.Format(os.Stdout, result, cfg.Output.Format); err != nil {
		return fmt.Errorf("failed to output results: %w", err)
	}

	// Check for failures based on risk level
	if result.HasFailures() {
		return fmt.Errorf("analysis failed: risk level exceeds threshold")
	}

	return nil
}

// detectDBType attempts to detect database type from connection URL
func detectDBType(url string) string {
	urlLower := strings.ToLower(url)

	if strings.HasPrefix(urlLower, "postgres://") || strings.HasPrefix(urlLower, "postgresql://") {
		return "postgresql"
	}

	if strings.Contains(urlLower, "mysql") || strings.Contains(urlLower, "@tcp") {
		return "mysql"
	}

	return ""
}
