package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/iamsr/dma/internal/analyzer"
	postgresanalyzer "github.com/iamsr/dma/internal/analyzer/postgres"
	"github.com/iamsr/dma/internal/config"
	"github.com/iamsr/dma/internal/db"
	"github.com/iamsr/dma/internal/introspector"
	"github.com/iamsr/dma/internal/output"
	"github.com/iamsr/dma/internal/parser"
	"github.com/iamsr/dma/pkg/models"
)

type analyzeOptions struct {
	dbURL           string
	dbType          string
	format          string
	dryRun          bool
	failOnRiskLevel string
	comprehensive   bool // Enable all Phase 2 features
}

func newAnalyzeCommand() *cobra.Command {
	opts := &analyzeOptions{}

	cmd := &cobra.Command{
		Use:   "analyze [migration-file-or-directory]",
		Short: "Analyze migration files for production impact",
		Long: `Analyze SQL migration files to predict production impact including:
- Lock types and durations
- Table rewrite requirements
- Index build times
- Backward compatibility issues
- Risk scoring and recommendations`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.dbURL, "db", "", "database connection URL")
	cmd.Flags().StringVar(&opts.dbType, "db-type", "", "database type (postgresql, mysql) - auto-detected from URL if not specified")
	cmd.Flags().StringVar(&opts.format, "format", "table", "output format (table, json, yaml)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "analyze without database connection")
	cmd.Flags().StringVar(&opts.failOnRiskLevel, "fail-on-risk-level", "", "exit with error if risk level exceeds threshold (low, medium, high, critical)")
	cmd.Flags().BoolVar(&opts.comprehensive, "comprehensive", false, "enable comprehensive analysis (dependencies, time breakdown, alternatives)")

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
	if opts.failOnRiskLevel != "" {
		cfg.Analysis.FailOnRiskLevel = opts.failOnRiskLevel
	}

	// Auto-detect database type from URL if not specified
	if cfg.Database.Type == "" && cfg.Database.URL != "" {
		cfg.Database.Type = detectDBType(cfg.Database.URL)
	}

	// If still no database type, default to postgresql
	if cfg.Database.Type == "" {
		cfg.Database.Type = "postgresql"
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Get parser
	sqlParser, err := parser.GetParser(cfg.Database.Type)
	if err != nil {
		return fmt.Errorf("failed to get parser: %w", err)
	}

	// Get introspector (optional in dry-run mode)
	var intr db.Introspector
	if !opts.dryRun && cfg.Database.URL != "" {
		intr, err = introspector.GetIntrospector(cfg.Database.Type, cfg.Database.URL)
		if err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to get introspector: %v\n", err)
		}
		if intr != nil {
			ctx := context.Background()
			if err := intr.Connect(ctx); err != nil {
				// Log warning but continue
				fmt.Fprintf(os.Stderr, "Warning: failed to connect to database: %v\n", err)
			} else {
				defer intr.Close()
			}
		}
	}

	// Find migration files
	files, err := findMigrationFiles(filePath)
	if err != nil {
		return fmt.Errorf("failed to find migration files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no migration files found in: %s", filePath)
	}

	// Parse migrations
	result := &models.AnalysisResult{
		Migrations:      make([]*models.Migration, 0),
		DatabaseType:    cfg.Database.Type,
		FailOnRiskLevel: models.RiskLevel(cfg.Analysis.FailOnRiskLevel),
		Errors:          []error{},
	}

	// Get analyzer for risk assessment
	// Even in dry-run mode, we can provide basic lock analysis with conservative estimates
	anlzr, err := analyzer.GetAnalyzer(cfg.Database.Type, intr, cfg.Analysis.DiskThroughputMBps, cfg.Analysis.RewriteFactor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to get analyzer: %v\n", err)
		anlzr = nil
	}

	ctx := context.Background()
	for _, file := range files {
		migration, err := sqlParser.ParseFile(file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to parse %s: %w", file, err))
			continue
		}

		// Analyze each operation for production impact
		if anlzr != nil {
			// Check if we're using comprehensive analysis (Phase 2 features)
			if opts.comprehensive {
				// Type assert to PostgreSQL analyzer to access AnalyzeWithEnhancements
				if pgAnalyzer, ok := anlzr.(*postgresanalyzer.Analyzer); ok {
					analysisOpts := postgresanalyzer.DefaultAnalysisOptions()
					for _, op := range migration.Operations {
						if err := pgAnalyzer.AnalyzeWithEnhancements(ctx, op, analysisOpts); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation in %s: %v\n", file, err)
						}
					}
				} else {
					// Fallback to basic analysis for non-PostgreSQL
					for _, op := range migration.Operations {
						if err := anlzr.Analyze(ctx, op); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation in %s: %v\n", file, err)
						}
					}
				}
			} else {
				// Use basic analysis
				for _, op := range migration.Operations {
					if err := anlzr.Analyze(ctx, op); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation in %s: %v\n", file, err)
					}
				}
			}
		}

		result.Migrations = append(result.Migrations, migration)
	}

	// If we have errors and no successful migrations, return error
	if len(result.Errors) > 0 && len(result.Migrations) == 0 {
		return fmt.Errorf("failed to parse migrations: %v", result.Errors[0])
	}

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

// findMigrationFiles finds migration files from a given path (file or directory)
func findMigrationFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		return []string{path}, nil
	}

	// Directory - find all .sql files
	var files []string
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(p), ".sql") {
			files = append(files, p)
		}
		return nil
	})

	return files, err
}
