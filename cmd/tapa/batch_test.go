package main

import (
	"testing"
)

func TestNewBatchCommand(t *testing.T) {
	cmd := newBatchCommand()

	if cmd.Use != "batch [migration-file-or-directory]" {
		t.Errorf("Expected 'batch' command, got %s", cmd.Use)
	}

	// Verify flags exist
	if cmd.Flags().Lookup("db") == nil {
		t.Error("Expected --db flag to exist")
	}
	if cmd.Flags().Lookup("db-type") == nil {
		t.Error("Expected --db-type flag to exist")
	}
	if cmd.Flags().Lookup("format") == nil {
		t.Error("Expected --format flag to exist")
	}
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Error("Expected --dry-run flag to exist")
	}
}
