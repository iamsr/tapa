package main

import (
	"github.com/spf13/cobra"
)

var (
	version = "0.3.0"
	cfgFile string
)

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dma",
		Short: "Database Migration Analyzer",
		Long: `Database Migration Analyzer (DMA) - Static analysis tool for predicting
database migration impact before execution.

Analyzes SQL migration files to provide insights on lock durations,
table rewrites, index build times, and backward compatibility.`,
		Version: version,
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .dma.yml)")

	// Add subcommands
	cmd.AddCommand(newAnalyzeCommand())

	return cmd
}
