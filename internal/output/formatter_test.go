package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/iamsr/tapa/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFormatTable(t *testing.T) {
	result := createTestResult()

	var buf bytes.Buffer
	err := FormatTable(&buf, result)
	require.NoError(t, err)

	output := buf.String()

	// Check for summary card
	assert.Contains(t, output, "ANALYSIS RESULTS")
	assert.Contains(t, output, "Risk Score")
	assert.Contains(t, output, "Risk Breakdown:")
	assert.Contains(t, output, "Compatibility:")

	// Check for operation card
	assert.Contains(t, output, "ADD_COLUMN on users")
	assert.Contains(t, output, "SQL:")
	assert.Contains(t, output, "Lock Analysis")
	assert.Contains(t, output, "Time Estimate")
	assert.Contains(t, output, "email")

	// Check for box drawing characters (new card style)
	assert.Contains(t, output, "╭")
	assert.Contains(t, output, "╮")
	assert.Contains(t, output, "╰")
	assert.Contains(t, output, "╯")
}

func TestFormatTable_EmptyResult(t *testing.T) {
	result := &models.AnalysisResult{
		Migrations:   []*models.Migration{},
		DatabaseType: "postgresql",
	}

	var buf bytes.Buffer
	err := FormatTable(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No migrations analyzed")
}

func TestFormatTable_WithErrors(t *testing.T) {
	result := createTestResult()
	result.Errors = []error{
		assert.AnError,
	}

	var buf bytes.Buffer
	err := FormatTable(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Errors")
}

func TestFormatJSON(t *testing.T) {
	result := createTestResult()

	var buf bytes.Buffer
	err := FormatJSON(&buf, result)
	require.NoError(t, err)

	// Verify valid JSON
	var decoded map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// Check structure (Go uses capitalized field names by default)
	assert.Contains(t, decoded, "Migrations")
	assert.Contains(t, decoded, "DatabaseType")

	migrations := decoded["Migrations"].([]interface{})
	assert.Len(t, migrations, 1)
}

func TestFormatYAML(t *testing.T) {
	result := createTestResult()

	var buf bytes.Buffer
	err := FormatYAML(&buf, result)
	require.NoError(t, err)

	// Verify valid YAML
	var decoded map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// Check structure (lowercase in YAML by default from struct tags or yaml encoder)
	assert.Contains(t, decoded, "migrations")
	assert.Contains(t, decoded, "databasetype")
}

func TestFormat_InvalidFormat(t *testing.T) {
	result := createTestResult()

	var buf bytes.Buffer
	err := Format(&buf, result, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestFormat_Table(t *testing.T) {
	result := createTestResult()

	var buf bytes.Buffer
	err := Format(&buf, result, "table")
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "ANALYSIS RESULTS")
	assert.Contains(t, output, "ADD_COLUMN on users")
}

func TestFormat_JSON(t *testing.T) {
	result := createTestResult()

	var buf bytes.Buffer
	err := Format(&buf, result, "json")
	require.NoError(t, err)

	// Verify valid JSON
	var decoded map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)
}

func TestFormat_YAML(t *testing.T) {
	result := createTestResult()

	var buf bytes.Buffer
	err := Format(&buf, result, "yaml")
	require.NoError(t, err)

	// Verify valid YAML
	var decoded map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)
}

// Helper function to create test data
func createTestResult() *models.AnalysisResult {
	migration := models.NewMigration("test_migration.sql")
	migration.AddOperation(&models.Operation{
		SQL:                  "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
		Type:                 models.OperationTypeAddColumn,
		TableName:            "users",
		LockType:             models.LockTypeAccessExclusive,
		LockDurationMS:       100,
		RequiresRewrite:      false,
		EstimatedTimeSeconds: 0.1,
		RiskScore:            25,
		BackwardCompatible:   true,
		Recommendations:      []string{"Column is nullable, backward compatible"},
	})

	return &models.AnalysisResult{
		Migrations:      []*models.Migration{migration},
		DatabaseType:    "postgresql",
		FailOnRiskLevel: models.RiskLevelHigh,
		Errors:          []error{},
	}
}
