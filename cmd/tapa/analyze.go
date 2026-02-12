package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iamsr/tapa/internal/analyzer"
	"github.com/iamsr/tapa/internal/analyzer/dryrun"
	mysqlanalyzer "github.com/iamsr/tapa/internal/analyzer/mysql"
	postgresanalyzer "github.com/iamsr/tapa/internal/analyzer/postgres"
	"github.com/iamsr/tapa/internal/config"
	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/internal/introspector"
	"github.com/iamsr/tapa/internal/output"
	"github.com/iamsr/tapa/internal/parser"
	"github.com/iamsr/tapa/internal/ui"
	"github.com/iamsr/tapa/pkg/models"
	"github.com/spf13/cobra"
)

type analyzeOptions struct {
	dbURL           string
	dbType          string
	format          string
	dryRun          bool
	dryRunDB        string // Database URL for dry-run execution testing
	failOnRiskLevel string
	comprehensive   bool // Enable all Phase 2 + advanced features (disk space, rollback, data migration)
	verbose         bool // Enable verbose output with progress indicators
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
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "analyze without database connection, or with temporary schema execution testing if --db is provided")
	cmd.Flags().StringVar(&opts.dryRunDB, "dry-run-db", "", "database URL for dry-run execution testing (defaults to --db)")
	cmd.Flags().StringVar(&opts.failOnRiskLevel, "fail-on-risk-level", "", "exit with error if risk level exceeds threshold (low, medium, high, critical)")
	cmd.Flags().BoolVar(&opts.comprehensive, "comprehensive", false, "enable comprehensive analysis (disk space, rollback, rollback, data migration, dependencies, time breakdown, alternatives)")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "enable verbose output with progress indicators")

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

	// Step 1: Database connection
	var intr db.Introspector
	if !opts.dryRun && cfg.Database.URL != "" {
		stepPrint(os.Stderr, "Connecting to database...")
		intr, err = introspector.GetIntrospector(cfg.Database.Type, cfg.Database.URL)
		if err != nil {
			stepWarn(os.Stderr, "Could not create introspector: %v", err)
		}
		if intr != nil {
			ctx := context.Background()
			if err := intr.Connect(ctx); err != nil {
				stepWarn(os.Stderr, "Could not connect to database: %v", err)
				intr = nil
			} else {
				defer intr.Close()
				dbLabel := strings.ToUpper(cfg.Database.Type[:1]) + cfg.Database.Type[1:]
				stepDone(os.Stderr, "Connected to %s", dbLabel)
			}
		}
	} else if opts.dryRun {
		stepDone(os.Stderr, "Dry-run mode (no database connection)")
	}

	// Setup dry-run analyzer if requested and DB URL available
	var dryRunAnalyzer *dryrun.Analyzer
	if opts.dryRun {
		// Determine which DB URL to use
		dryRunDBURL := cfg.Database.URL
		if opts.dryRunDB != "" {
			dryRunDBURL = opts.dryRunDB
		}

		// Only create analyzer if we have a DB URL
		if dryRunDBURL != "" {
			dryRunConn, err := sql.Open(cfg.Database.Type, dryRunDBURL)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to connect for dry-run testing: %v\n", err)
			} else {
				defer dryRunConn.Close()
				dryRunAnalyzer = dryrun.NewAnalyzer(cfg.Database.Type, dryRunConn)

				if opts.verbose {
					fmt.Fprintln(os.Stderr, "✓ Dry-run execution testing enabled")
				}
			}
		}
	}

	// Get parser
	sqlParser, err := parser.GetParser(cfg.Database.Type)
	if err != nil {
		return fmt.Errorf("failed to get parser: %w", err)
	}

	// Find migration files
	files, err := findMigrationFiles(filePath)
	if err != nil {
		return fmt.Errorf("failed to find migration files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no migration files found in: %s", filePath)
	}

	// Step 2: Parse migration files
	stepPrint(os.Stderr, "Parsing migration file(s)...")

	result := &models.AnalysisResult{
		Migrations:      make([]*models.Migration, 0),
		DatabaseType:    cfg.Database.Type,
		FailOnRiskLevel: models.RiskLevel(cfg.Analysis.FailOnRiskLevel),
		Errors:          []error{},
	}

	// Get analyzer for risk assessment
	var anlzr analyzer.Analyzer
	var pgAnalyzer *postgresanalyzer.Analyzer
	var mysqlAnalyzer *mysqlanalyzer.Analyzer

	switch cfg.Database.Type {
	case "postgresql":
		pgAnalyzer = postgresanalyzer.NewAnalyzer(intr, cfg.Analysis.DiskThroughputMBps, cfg.Analysis.RewriteFactor, opts.comprehensive)
		anlzr = pgAnalyzer
	case "mysql":
		// MySQL analyzer doesn't support comprehensive mode yet
		mysqlAnalyzer = mysqlanalyzer.NewAnalyzer(intr, cfg.Analysis.DiskThroughputMBps, cfg.Analysis.RewriteFactor)
		anlzr = mysqlAnalyzer
	default:
		fmt.Fprintf(os.Stderr, "Warning: unsupported database type: %s\n", cfg.Database.Type)
	}

	ctx := context.Background()
	operationCount := 0
	for _, file := range files {
		migration, err := sqlParser.ParseFile(file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to parse %s: %w", file, err))
			continue
		}

		operationCount += len(migration.Operations)
		result.Migrations = append(result.Migrations, migration)
	}

	if operationCount == 1 {
		stepDone(os.Stderr, "Found %d statement", operationCount)
	} else {
		stepDone(os.Stderr, "Found %d statements", operationCount)
	}

	// Step 3: Analyze operations
	if anlzr != nil && operationCount > 0 {
		stepPrint(os.Stderr, "Analyzing operations...")

		for _, migration := range result.Migrations {
			if opts.comprehensive {
				if pgAnalyzer != nil {
					analysisOpts := postgresanalyzer.DefaultAnalysisOptions()
					for _, op := range migration.Operations {
						if err := pgAnalyzer.AnalyzeWithEnhancements(ctx, op, analysisOpts); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation: %v\n", err)
						}

						// Execute dry-run if analyzer is available
						if dryRunAnalyzer != nil {
							if opts.verbose {
								fmt.Fprintf(os.Stderr, "  Running execution test for %s...\n", op.Type)
							}

							if err := dryRunAnalyzer.AnalyzeOperation(ctx, op); err != nil {
								// Non-fatal: continue with analysis
								fmt.Fprintf(os.Stderr, "Warning: Dry-run execution test failed for %s: %v\n", op.Type, err)
							}
						}
					}
				} else if mysqlAnalyzer != nil {
					analysisOpts := mysqlanalyzer.DefaultAnalysisOptions()
					for _, op := range migration.Operations {
						if err := mysqlAnalyzer.AnalyzeWithEnhancements(ctx, op, analysisOpts); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation: %v\n", err)
						}

						// Execute dry-run if analyzer is available
						if dryRunAnalyzer != nil {
							if opts.verbose {
								fmt.Fprintf(os.Stderr, "  Running execution test for %s...\n", op.Type)
							}

							if err := dryRunAnalyzer.AnalyzeOperation(ctx, op); err != nil {
								// Non-fatal: continue with analysis
								fmt.Fprintf(os.Stderr, "Warning: Dry-run execution test failed for %s: %v\n", op.Type, err)
							}
						}
					}
				} else {
					for _, op := range migration.Operations {
						if err := anlzr.Analyze(ctx, op); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation: %v\n", err)
						}

						// Execute dry-run if analyzer is available
						if dryRunAnalyzer != nil {
							if opts.verbose {
								fmt.Fprintf(os.Stderr, "  Running execution test for %s...\n", op.Type)
							}

							if err := dryRunAnalyzer.AnalyzeOperation(ctx, op); err != nil {
								// Non-fatal: continue with analysis
								fmt.Fprintf(os.Stderr, "Warning: Dry-run execution test failed for %s: %v\n", op.Type, err)
							}
						}
					}
				}
			} else {
				for _, op := range migration.Operations {
					if err := anlzr.Analyze(ctx, op); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation: %v\n", err)
					}

					// Execute dry-run if analyzer is available
					if dryRunAnalyzer != nil {
						if opts.verbose {
							fmt.Fprintf(os.Stderr, "  Running execution test for %s...\n", op.Type)
						}

						if err := dryRunAnalyzer.AnalyzeOperation(ctx, op); err != nil {
							// Non-fatal: continue with analysis
							fmt.Fprintf(os.Stderr, "Warning: Dry-run execution test failed for %s: %v\n", op.Type, err)
						}
					}
				}
			}
		}

		stepDone(os.Stderr, "Analysis complete")
	}

	fmt.Fprintln(os.Stderr) // blank line before results

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

// stepPrint prints a progress step message to stderr
func stepPrint(w *os.File, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "  %s\n", msg)
}

// stepDone prints a completed step with a checkmark to stderr
func stepDone(w *os.File, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "  %s %s\n", ui.StepCheck(), msg)
}

// stepWarn prints a warning step to stderr
func stepWarn(w *os.File, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "  %s %s\n", ui.StepWarn(), msg)
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
