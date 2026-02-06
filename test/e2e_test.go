package test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestE2E_PostgreSQL_Analysis tests PostgreSQL parsing and analysis end-to-end
func TestE2E_PostgreSQL_Analysis(t *testing.T) {
	// Build the TAPA binary if not exists
	cmd := exec.Command("go", "build", "-o", "../tapa", "../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run analysis on PostgreSQL migration
	cmd = exec.Command("../tapa", "analyze", "../examples/001_complex_migration.sql", "--dry-run", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\n%s", err, output)
	}

	result := string(output)

	// Verify JSON output is valid
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
		t.Fatalf("Invalid JSON output: %v\n%s", err, result)
	}

	// Check for Migrations field
	if _, ok := jsonResult["Migrations"]; !ok {
		t.Error("Expected JSON output with Migrations field")
	}

	t.Logf("PostgreSQL analysis output: %s", result)
}

// TestE2E_MySQL_Analysis tests MySQL parsing and analysis end-to-end
func TestE2E_MySQL_Analysis(t *testing.T) {
	// Build the TAPA binary if not exists
	cmd := exec.Command("go", "build", "-o", "../tapa", "../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run analysis on MySQL migration
	cmd = exec.Command("../tapa", "analyze", "../examples/mysql_migration.sql", "--dry-run", "--db-type", "mysql", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\n%s", err, output)
	}

	result := string(output)

	// Verify JSON output is valid
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
		t.Fatalf("Invalid JSON output: %v\n%s", err, result)
	}

	// Check for Migrations field
	if _, ok := jsonResult["Migrations"]; !ok {
		t.Error("Expected JSON output with Migrations field")
	}

	// Check for ADD_COLUMN operation
	if !strings.Contains(result, "ADD_COLUMN") {
		t.Error("Expected MySQL analysis to contain ADD_COLUMN operation")
	}

	t.Logf("MySQL analysis output: %s", result)
}

// TestE2E_Comprehensive_Analysis tests Phase 2 comprehensive features
func TestE2E_Comprehensive_Analysis(t *testing.T) {
	// Build the TAPA binary if not exists
	cmd := exec.Command("go", "build", "-o", "../tapa", "../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run comprehensive analysis
	cmd = exec.Command("../tapa", "analyze", "../examples/001_complex_migration.sql", "--dry-run", "--comprehensive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\n%s", err, output)
	}

	result := string(output)

	// Check for comprehensive features (detailed output with operation details)
	if !strings.Contains(result, "Operation Details") && !strings.Contains(result, "Operations Detected") {
		t.Error("Expected comprehensive analysis to contain detailed operation information")
	}

	t.Logf("Comprehensive analysis output: %s", result)
}
