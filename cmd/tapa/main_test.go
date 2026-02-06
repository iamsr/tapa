package main

import (
	"testing"
)

func TestMainExists(t *testing.T) {
	// Test that main package compiles and links correctly.
	// We don't call main() directly because it calls os.Exit(0),
	// which would terminate the test process.
	// The existence of this test file ensures the main package is valid.
}
