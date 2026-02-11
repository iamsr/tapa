package main

import (
	"context"
	"fmt"
	"os"

	mysqlanalyzer "github.com/iamsr/tapa/internal/analyzer/mysql"
	postgresanalyzer "github.com/iamsr/tapa/internal/analyzer/postgres"
	"github.com/iamsr/tapa/internal/config"
	"github.com/iamsr/tapa/internal/db"
	"github.com/iamsr/tapa/internal/introspector"
	"github.com/iamsr/tapa/internal/output"
	"github.com/iamsr/tapa/internal/parser"
	"github.com/iamsr/tapa/pkg/models"
	"github.com/spf13/cobra"
)

type batchOptions struct {
	dbURL  string
	dbType string
	format string
	dryRun bool
}

func newBatchCommand() *cobra.Command {
	opts := &batchOptions{}

	cmd := &cobra.Command{
		Use:   "batch [migration-file-or-directory]",
		Short: "Generate batching strategy for migration operations",
		Long: `Analyze migration files and generate a batching strategy that groups
operations by risk level for safer incremental deployment.

Low-risk operations are batched together for parallel execution.
Medium-risk operations are executed sequentially.
High/critical-risk operations are isolated in individual batches.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBatch(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.dbURL, "db", "", "database connection URL")
	cmd.Flags().StringVar(&opts.dbType, "db-type", "", "database type (postgresql, mysql)")
	cmd.Flags().StringVar(&opts.format, "format", "table", "output format (table, json, yaml)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "analyze without database connection")

	return cmd
}

func runBatch(filePath string, opts *batchOptions) error {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override with CLI flags
	if opts.dbURL != "" {
		cfg.Database.URL = opts.dbURL
	}
	if opts.dbType != "" {
		cfg.Database.Type = opts.dbType
	}
	if opts.format != "" {
		cfg.Output.Format = opts.format
	}

	// Auto-detect database type
	if cfg.Database.Type == "" && cfg.Database.URL != "" {
		cfg.Database.Type = detectDBType(cfg.Database.URL)
	}
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

	// Get introspector (optional)
	var intr db.Introspector
	if !opts.dryRun && cfg.Database.URL != "" {
		intr, err = introspector.GetIntrospector(cfg.Database.Type, cfg.Database.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get introspector: %v\n", err)
		}
		if intr != nil {
			ctx := context.Background()
			if err := intr.Connect(ctx); err != nil {
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

	// Create analyzer once
	var analyzer interface {
		Analyze(context.Context, *models.Operation) error
		BatchOperations([]*models.Operation) (*models.BatchingStrategy, error)
	}

	switch cfg.Database.Type {
	case "postgresql":
		analyzer = postgresanalyzer.NewAnalyzer(intr, cfg.Analysis.DiskThroughputMBps, cfg.Analysis.RewriteFactor, false)
	case "mysql":
		analyzer = mysqlanalyzer.NewAnalyzer(intr, cfg.Analysis.DiskThroughputMBps, cfg.Analysis.RewriteFactor)
	default:
		return fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	// Parse and analyze all operations
	var allOperations []*models.Operation
	ctx := context.Background()

	for _, file := range files {
		migration, err := sqlParser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", file, err)
			continue
		}

		// Analyze each operation
		for _, op := range migration.Operations {
			if err := analyzer.Analyze(ctx, op); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to analyze operation: %v\n", err)
			}

			allOperations = append(allOperations, op)
		}
	}

	if len(allOperations) == 0 {
		return fmt.Errorf("no operations found to batch")
	}

	// Generate batching strategy
	strategy, err := analyzer.BatchOperations(allOperations)
	if err != nil {
		return fmt.Errorf("failed to generate batching strategy: %w", err)
	}

	// Output batching strategy
	result := &models.BatchResult{
		Strategy:     strategy,
		DatabaseType: cfg.Database.Type,
	}

	if err := output.FormatBatching(os.Stdout, result, cfg.Output.Format); err != nil {
		return fmt.Errorf("failed to output results: %w", err)
	}

	return nil
}
