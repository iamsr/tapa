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
		Use:   "tapa",
		Short: "Table Alteration Planning Assistant",
		Long: `Table Alteration Planning Assistant (TAPA) - Static analysis tool for predicting
database migration impact before execution.

Analyzes SQL migration files to provide insights on lock durations,
table rewrites, index build times, and backward compatibility.`,
		Version: version,
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .tapa.yml)")

	// Add subcommands
	cmd.AddCommand(newAnalyzeCommand())
	cmd.AddCommand(newBatchCommand())

	return cmd
}
