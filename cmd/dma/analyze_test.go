package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCommand_Structure(t *testing.T) {
	cmd := newAnalyzeCommand()

	assert.Equal(t, "analyze [migration-file-or-directory]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotNil(t, cmd.RunE)
}

func TestAnalyzeCommand_Flags(t *testing.T) {
	cmd := newAnalyzeCommand()

	assert.NotNil(t, cmd.Flags().Lookup("db"))
	assert.NotNil(t, cmd.Flags().Lookup("db-type"))
	assert.NotNil(t, cmd.Flags().Lookup("format"))
	assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
	assert.NotNil(t, cmd.Flags().Lookup("fail-on-risk-level"))
}

func TestAnalyzeCommand_MissingArgument(t *testing.T) {
	cmd := newAnalyzeCommand()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestAnalyzeCommand_NonExistentFile(t *testing.T) {
	cmd := newAnalyzeCommand()
	cmd.SetArgs([]string{"/nonexistent/file.sql"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestAnalyzeCommand_ValidSQLFile(t *testing.T) {
	// Create temp SQL file
	tmpDir := t.TempDir()
	sqlFile := filepath.Join(tmpDir, "001_test.sql")
	sqlContent := `-- Test migration
ALTER TABLE users ADD COLUMN email VARCHAR(255);
CREATE INDEX idx_users_email ON users(email);
`
	err := os.WriteFile(sqlFile, []byte(sqlContent), 0644)
	require.NoError(t, err)

	cmd := newAnalyzeCommand()
	// Use postgresql as default type
	cmd.SetArgs([]string{sqlFile, "--db-type", "postgresql"})

	err = cmd.Execute()
	// Should succeed in parsing (even without database connection in current MVP)
	assert.NoError(t, err)
}

func TestAnalyzeCommand_InvalidSQL(t *testing.T) {
	// Create temp SQL file with invalid SQL
	tmpDir := t.TempDir()
	sqlFile := filepath.Join(tmpDir, "bad.sql")
	err := os.WriteFile(sqlFile, []byte("INVALID SQL SYNTAX;;;"), 0644)
	require.NoError(t, err)

	cmd := newAnalyzeCommand()
	cmd.SetArgs([]string{sqlFile, "--db-type", "postgresql"})

	err = cmd.Execute()
	// Parser should return an error for invalid SQL
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestAnalyzeCommand_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	sqlFile := filepath.Join(tmpDir, "test.sql")
	err := os.WriteFile(sqlFile, []byte("CREATE TABLE test (id INT);"), 0644)
	require.NoError(t, err)

	cmd := newAnalyzeCommand()
	cmd.SetArgs([]string{sqlFile, "--db-type", "postgresql", "--format", "json"})

	err = cmd.Execute()
	assert.NoError(t, err)
}

func TestAnalyzeCommand_YAMLFormat(t *testing.T) {
	tmpDir := t.TempDir()
	sqlFile := filepath.Join(tmpDir, "test.sql")
	err := os.WriteFile(sqlFile, []byte("CREATE TABLE test (id INT);"), 0644)
	require.NoError(t, err)

	cmd := newAnalyzeCommand()
	cmd.SetArgs([]string{sqlFile, "--db-type", "postgresql", "--format", "yaml"})

	err = cmd.Execute()
	assert.NoError(t, err)
}

func TestAnalyzeCommand_DryRun(t *testing.T) {
	// Create temp SQL file
	tmpDir := t.TempDir()
	sqlFile := filepath.Join(tmpDir, "001_test.sql")
	err := os.WriteFile(sqlFile, []byte("ALTER TABLE users ADD COLUMN email VARCHAR(255);"), 0644)
	require.NoError(t, err)

	cmd := newAnalyzeCommand()
	cmd.SetArgs([]string{sqlFile, "--dry-run"})

	err = cmd.Execute()
	// Should not fail in dry-run mode without database
	assert.NoError(t, err)
}

func TestAnalyzeCommand_MySQL_DryRun(t *testing.T) {
	// Create temp directory for migration file
	tmpDir := t.TempDir()
	sqlFile := filepath.Join(tmpDir, "mysql_migration.sql")

	// Create test migration file with MySQL SQL
	sqlContent := `-- Test MySQL migration
ALTER TABLE users ADD COLUMN email VARCHAR(255);
CREATE INDEX idx_email ON users(email) ALGORITHM=INPLACE;
`
	err := os.WriteFile(sqlFile, []byte(sqlContent), 0644)
	require.NoError(t, err)

	// Build analyze command with MySQL-specific args
	cmd := newAnalyzeCommand()
	cmd.SetArgs([]string{
		sqlFile,
		"--dry-run",
		"--db-type", "mysql",
	})

	// Execute command
	err = cmd.Execute()

	// Verify success (no error)
	// This verifies the full CLI workflow:
	// - SQL file is read
	// - MySQL parser is used (--db-type mysql)
	// - Analysis is performed
	// - Output is generated
	assert.NoError(t, err, "MySQL CLI analysis should complete without error")
}
