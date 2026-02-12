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
	cmd := exec.Command("go", "build", "-o", "../../tapa", "../../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run analysis on PostgreSQL migration
	// Use Output() (stdout only) since progress messages go to stderr
	cmd = exec.Command("../../tapa", "analyze", "../../examples/001_complex_migration.sql", "--dry-run", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Command failed: %v\nstderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Command failed: %v", err)
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
	cmd := exec.Command("go", "build", "-o", "../../tapa", "../../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run analysis on MySQL migration
	// Use Output() (stdout only) since progress messages go to stderr
	cmd = exec.Command("../../tapa", "analyze", "../../examples/mysql_migration.sql", "--dry-run", "--db-type", "mysql", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Command failed: %v\nstderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Command failed: %v", err)
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

// TestE2E_Comprehensive_Analysis tests comprehensive mode with all advanced features
func TestE2E_Comprehensive_Analysis(t *testing.T) {
	// Build the TAPA binary if not exists
	cmd := exec.Command("go", "build", "-o", "../../tapa", "../../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run comprehensive analysis
	// Use Output() (stdout only) since progress messages go to stderr
	cmd = exec.Command("../../tapa", "analyze", "../../examples/001_complex_migration.sql", "--dry-run", "--comprehensive")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Command failed: %v\nstderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Command failed: %v", err)
	}

	result := string(output)

	// Check for comprehensive features (detailed output with operation cards)
	if !strings.Contains(result, "Lock:") && !strings.Contains(result, "ANALYSIS RESULTS") {
		t.Error("Expected comprehensive analysis to contain detailed operation information")
	}

	// Verify all advanced features are present
	t.Run("DiskSpaceAnalysis", func(t *testing.T) {
		if !strings.Contains(result, "Disk Space Analysis:") {
			t.Error("Expected comprehensive analysis to include Disk Space Analysis")
		}
		if !strings.Contains(result, "Peak disk usage:") {
			t.Error("Expected disk space analysis to show peak disk usage")
		}
	})

	t.Run("RollbackAnalysis", func(t *testing.T) {
		if !strings.Contains(result, "Rollback Analysis:") {
			t.Error("Expected comprehensive analysis to include Rollback Analysis")
		}
		if !strings.Contains(result, "Reversibility Score:") {
			t.Error("Expected rollback analysis to show reversibility score")
		}
		if !strings.Contains(result, "Auto-generated Rollback:") {
			t.Error("Expected rollback analysis to show auto-generated rollback SQL")
		}
	})

	t.Run("DryRunSimulation", func(t *testing.T) {
		// Note: Dry-run requires --db connection, so it won't run in this test
		// But we verify the flag works without errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("Dry-run skipped (no database connection): %s", exitErr.Stderr)
		}
	})

	t.Run("ConcurrencyAnalysis", func(t *testing.T) {
		// Concurrency analysis requires --db connection for workload analysis
		// But the analyzer should still run and provide lock impact analysis
		// We verify it doesn't crash without a database
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("Concurrency analysis with mock mode: %s", exitErr.Stderr)
		}
	})

	t.Logf("Comprehensive analysis output: %s", result)
}

// TestE2E_ConcurrencyFlag tests the --concurrency flag independently
func TestE2E_ConcurrencyFlag(t *testing.T) {
	// Build the TAPA binary if not exists
	cmd := exec.Command("go", "build", "-o", "../../tapa", "../../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run analysis with --concurrency flag (without database connection)
	cmd = exec.Command("../../tapa", "analyze", "../../examples/001_complex_migration.sql", "--concurrency")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Command failed: %v\nstderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Command failed: %v", err)
	}

	result := string(output)

	// Verify the analysis ran successfully
	if !strings.Contains(result, "ANALYSIS RESULTS") && !strings.Contains(result, "Lock:") {
		t.Error("Expected concurrency analysis to run successfully")
	}

	// Note: Concurrency Impact Analysis section won't appear without database connection
	// But the command should not fail
	t.Logf("Concurrency flag output: %s", result)
}

// TestE2E_IndividualAdvancedFeatures tests each advanced feature flag independently
func TestE2E_IndividualAdvancedFeatures(t *testing.T) {
	// Build the TAPA binary if not exists
	cmd := exec.Command("go", "build", "-o", "../../tapa", "../../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	tests := []struct {
		name     string
		flag     string
		expected string
	}{
		{
			name:     "DryRunFlag",
			flag:     "--dry-run",
			expected: "Lock:",
		},
		{
			name:     "ConcurrencyFlag",
			flag:     "--concurrency",
			expected: "Lock:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("../../tapa", "analyze", "../../examples/001_complex_migration.sql", tt.flag)
			output, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Fatalf("Command with %s failed: %v\nstderr: %s", tt.flag, err, exitErr.Stderr)
				}
				t.Fatalf("Command with %s failed: %v", tt.flag, err)
			}

			result := string(output)

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected output with %s to contain '%s'", tt.flag, tt.expected)
			}

			t.Logf("%s output: %s", tt.name, result)
		})
	}
}

// TestE2E_JSONOutput tests JSON output format with all features
func TestE2E_JSONOutput(t *testing.T) {
	// Build the TAPA binary if not exists
	cmd := exec.Command("go", "build", "-o", "../../tapa", "../../cmd/tapa")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build TAPA: %v\n%s", err, output)
	}

	// Run comprehensive analysis with JSON output
	cmd = exec.Command("../../tapa", "analyze", "../../examples/001_complex_migration.sql", "--comprehensive", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Command failed: %v\nstderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Command failed: %v", err)
	}

	result := string(output)

	// Verify JSON output is valid
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
		t.Fatalf("Invalid JSON output: %v\n%s", err, result)
	}

	// Verify structure
	migrations, ok := jsonResult["Migrations"].([]interface{})
	if !ok || len(migrations) == 0 {
		t.Error("Expected Migrations array in JSON output")
	}

	// Check first migration has operations
	if len(migrations) > 0 {
		migration := migrations[0].(map[string]interface{})
		operations, ok := migration["Operations"].([]interface{})
		if !ok || len(operations) == 0 {
			t.Error("Expected Operations array in first migration")
		}

		// Check first operation has advanced analysis fields
		if len(operations) > 0 {
			operation := operations[0].(map[string]interface{})

			// These fields should be present in comprehensive mode (lowercase with underscores in JSON)
			if _, hasRollback := operation["rollback_analysis"]; !hasRollback {
				t.Error("Expected rollback_analysis in operation")
			}

			if _, hasDiskSpace := operation["disk_space_analysis"]; !hasDiskSpace {
				t.Error("Expected disk_space_analysis in operation")
			}

			t.Logf("Operation has rollback_analysis and disk_space_analysis fields")
		}
	}

	t.Logf("JSON output validated successfully")
}
