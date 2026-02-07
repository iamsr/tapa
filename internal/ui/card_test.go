package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/iamsr/tapa/pkg/models"
)

func TestDrawBox(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		lines    []string
		width    int
		wantLen  int // approximate line length
		contains []string
	}{
		{
			name:     "simple box with title",
			title:    "Test",
			lines:    []string{"Line 1", "Line 2"},
			width:    30,
			wantLen:  30,
			contains: []string{"Test", "Line 1", "Line 2", "╭", "╮", "╰", "╯", "│"},
		},
		{
			name:     "box without title",
			title:    "",
			lines:    []string{"Content"},
			width:    20,
			wantLen:  20,
			contains: []string{"Content", "╭", "╮", "╰", "╯"},
		},
		{
			name:     "box with multiple lines",
			title:    "Multi",
			lines:    []string{"A", "B", "C"},
			width:    25,
			wantLen:  25,
			contains: []string{"Multi", "A", "B", "C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DrawBox(tt.title, tt.lines, tt.width)

			// Check if result contains expected strings
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("DrawBox() result missing %q\nGot:\n%s", s, result)
				}
			}

			// Check line count (title line + content lines + bottom line)
			lines := strings.Split(result, "\n")
			expectedLines := 1 + len(tt.lines) + 1
			if len(lines) != expectedLines {
				t.Errorf("DrawBox() line count = %d, want %d", len(lines), expectedLines)
			}
		})
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no ansi codes",
			input: "plain text",
			want:  "plain text",
		},
		{
			name:  "with color codes",
			input: "\x1b[31mred text\x1b[0m",
			want:  "red text",
		},
		{
			name:  "multiple colors",
			input: "\x1b[32mgreen\x1b[0m and \x1b[31mred\x1b[0m",
			want:  "green and red",
		},
		{
			name:  "bold text",
			input: "\x1b[1mbold\x1b[0m",
			want:  "bold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAnsi(tt.input)
			if got != tt.want {
				t.Errorf("stripAnsi() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSummaryCard(t *testing.T) {
	// Create a test migration
	migration := models.NewMigration("test_migration.sql")
	migration.AddOperation(&models.Operation{
		Type:                 models.OperationTypeAlterTable,
		TableName:            "users",
		RiskScore:            30,
		EstimatedTimeSeconds: 5.5,
		BackwardCompatible:   true,
	})
	migration.AddOperation(&models.Operation{
		Type:                 models.OperationTypeAddColumn,
		TableName:            "posts",
		RiskScore:            80,
		EstimatedTimeSeconds: 120.0,
		BackwardCompatible:   false,
	})

	result := FormatSummaryCard(migration)

	// Summary card should contain aggregate-level info only
	expected := []string{
		"ANALYSIS RESULTS",
		"Risk Score",
		"Status:",
		"Est. Time:",
		"Risk Breakdown:",
		"Low",
		"Medium",
		"High",
		"Critical",
		"Compatibility:",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("FormatSummaryCard() missing %q\nGot:\n%s", exp, result)
		}
	}

	// Summary card should NOT contain per-operation details
	shouldNotContain := []string{
		"Lock Analysis",
		"Time Estimate",
		"Execution",
		"Table Size",
	}

	for _, s := range shouldNotContain {
		if strings.Contains(result, s) {
			t.Errorf("FormatSummaryCard() should not contain per-operation field %q\nGot:\n%s", s, result)
		}
	}

	// Check for box drawing characters
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╮") {
		t.Errorf("FormatSummaryCard() missing box borders")
	}
}

func TestFormatSummaryCardBottomMessage(t *testing.T) {
	tests := []struct {
		name      string
		riskScore int
		contains  string
	}{
		{"safe to deploy", 10, "SAFE TO DEPLOY"},
		{"standard precautions", 30, "Standard precautions"},
		{"requires staging", 60, "staging environment"},
		{"critical review", 80, "CRITICAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := models.NewMigration("test.sql")
			migration.AddOperation(&models.Operation{
				RiskScore:          tt.riskScore,
				BackwardCompatible: true,
			})
			result := FormatSummaryCard(migration)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("FormatSummaryCard() with risk %d missing %q", tt.riskScore, tt.contains)
			}
		})
	}
}

func TestFormatOperationCard(t *testing.T) {
	op := &models.Operation{
		Type:                 models.OperationTypeAlterTable,
		TableName:            "users",
		SQL:                  "ALTER TABLE users ADD COLUMN phone VARCHAR(20);",
		RiskScore:            15,
		LockType:             models.LockTypeAccessExclusive,
		LockDurationMS:       5,
		EstimatedTimeSeconds: 0.5,
		BackwardCompatible:   true,
		RequiresRewrite:      false,
		Recommendations:      []string{"Consider using IF NOT EXISTS"},
	}

	result := FormatOperationCard(op, 1)

	// Operation card should contain per-operation details
	expected := []string{
		"ALTER_TABLE on users",
		"SQL:",
		"Risk Score",
		"Status:",
		"Lock Analysis",
		"Type",
		"Duration",
		"Queries",
		"Time Estimate",
		"Execution",
		"Table Size",
		"Compatibility",
		"Backward compatible",
		"No table rewrite",
		"Recommendations",
		"IF NOT EXISTS",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("FormatOperationCard() missing %q\nGot:\n%s", exp, result)
		}
	}

	// Check box borders
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╮") {
		t.Errorf("FormatOperationCard() missing box borders")
	}

	// Verify all lines have matching left and right borders
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		stripped := stripAnsi(line)
		if stripped == "" {
			continue
		}
		runes := []rune(stripped)
		if i == 0 || i == len(lines)-1 {
			// Top/bottom borders
			continue
		}
		if runes[0] != '│' {
			t.Errorf("Line %d missing left border: %q", i, stripped)
		}
		if runes[len(runes)-1] != '│' {
			t.Errorf("Line %d missing right border: %q", i, stripped)
		}
	}
}

func TestVisibleWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"plain ascii", "hello", 5},
		{"with ANSI color", "\x1b[31mred\x1b[0m", 3},
		{"emoji wide char", "🟢", 2},
		{"sparkle emoji", "✨", 2},
		{"check mark (not wide)", "✓", 1},
		{"mixed content", "Status: \x1b[32mLOW RISK\x1b[0m 🟢", 19},
		{"box drawing (not wide)", "├── Type", 8},
		{"empty string", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleWidth(tt.input)
			if got != tt.want {
				t.Errorf("visibleWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	migration := models.NewMigration("test.sql")
	migration.AddOperation(&models.Operation{
		RiskScore:            10,
		EstimatedTimeSeconds: 1.0,
		BackwardCompatible:   true,
	})
	migration.AddOperation(&models.Operation{
		RiskScore:            30,
		EstimatedTimeSeconds: 2.0,
		BackwardCompatible:   true,
	})
	migration.AddOperation(&models.Operation{
		RiskScore:            60,
		EstimatedTimeSeconds: 3.0,
		BackwardCompatible:   false,
	})

	summary := calculateSummary(migration)

	if summary.TotalOps != 3 {
		t.Errorf("TotalOps = %d, want 3", summary.TotalOps)
	}
	if summary.LowRisk != 1 {
		t.Errorf("LowRisk = %d, want 1", summary.LowRisk)
	}
	if summary.MediumRisk != 1 {
		t.Errorf("MediumRisk = %d, want 1", summary.MediumRisk)
	}
	if summary.HighRisk != 1 {
		t.Errorf("HighRisk = %d, want 1", summary.HighRisk)
	}
	if summary.BackwardCompatible != 2 {
		t.Errorf("BackwardCompatible = %d, want 2", summary.BackwardCompatible)
	}
	if summary.MaxRiskScore != 60 {
		t.Errorf("MaxRiskScore = %d, want 60", summary.MaxRiskScore)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 5 * time.Second, "5.0s"},
		{"minutes", 2 * time.Minute, "2.0m"},
		{"hours", 3 * time.Hour, "3.0h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColorsEnabled(t *testing.T) {
	// Save original env vars
	origNoColor := os.Getenv("NO_COLOR")
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("NO_COLOR", origNoColor)
		os.Setenv("TERM", origTerm)
	}()

	// In test environment, stdout is not a TTY, so colors are always disabled
	// unless we explicitly test NO_COLOR behavior
	os.Setenv("NO_COLOR", "1")
	os.Setenv("TERM", "xterm-256color")
	if colorsEnabled() {
		t.Errorf("colorsEnabled() = true with NO_COLOR=1, want false")
	}

	os.Setenv("NO_COLOR", "")
	os.Setenv("TERM", "dumb")
	if colorsEnabled() {
		t.Errorf("colorsEnabled() = true with TERM=dumb, want false")
	}

	os.Setenv("NO_COLOR", "")
	os.Setenv("TERM", "")
	if colorsEnabled() {
		t.Errorf("colorsEnabled() = true with TERM='', want false")
	}
}

func TestColorize(t *testing.T) {
	// Save and restore NO_COLOR
	origNoColor := os.Getenv("NO_COLOR")
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("NO_COLOR", origNoColor)
		os.Setenv("TERM", origTerm)
	}()

	// Force colors off
	os.Setenv("NO_COLOR", "1")
	os.Setenv("TERM", "xterm-256color")

	got := colorize("error", ansiRed)
	if got != "error" {
		t.Errorf("colorize() with NO_COLOR=1 = %q, want %q", got, "error")
	}

	got = bold("heading")
	if got != "heading" {
		t.Errorf("bold() with NO_COLOR=1 = %q, want %q", got, "heading")
	}
}

func TestEmojisEnabled(t *testing.T) {
	origNoEmoji := os.Getenv("TAPA_NO_EMOJI")
	defer os.Setenv("TAPA_NO_EMOJI", origNoEmoji)

	os.Setenv("TAPA_NO_EMOJI", "")
	if !emojisEnabled() {
		t.Errorf("emojisEnabled() = false without TAPA_NO_EMOJI, want true")
	}

	os.Setenv("TAPA_NO_EMOJI", "1")
	if emojisEnabled() {
		t.Errorf("emojisEnabled() = true with TAPA_NO_EMOJI=1, want false")
	}
}

func TestEmoji(t *testing.T) {
	origNoEmoji := os.Getenv("TAPA_NO_EMOJI")
	defer os.Setenv("TAPA_NO_EMOJI", origNoEmoji)

	os.Setenv("TAPA_NO_EMOJI", "")
	got := emoji("🟢", "[+]")
	if got != "🟢" {
		t.Errorf("emoji() without TAPA_NO_EMOJI = %q, want %q", got, "🟢")
	}

	os.Setenv("TAPA_NO_EMOJI", "1")
	got = emoji("🟢", "[+]")
	if got != "[+]" {
		t.Errorf("emoji() with TAPA_NO_EMOJI=1 = %q, want %q", got, "[+]")
	}
}

func TestFormatSummaryCardNoColor(t *testing.T) {
	// Save and restore env vars
	origNoColor := os.Getenv("NO_COLOR")
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("NO_COLOR", origNoColor)
		os.Setenv("TERM", origTerm)
	}()

	os.Setenv("NO_COLOR", "1")
	os.Setenv("TERM", "xterm-256color")

	migration := models.NewMigration("test.sql")
	migration.AddOperation(&models.Operation{
		RiskScore:          15,
		BackwardCompatible: true,
	})

	result := FormatSummaryCard(migration)

	// With NO_COLOR=1, output should have NO ANSI escape codes
	if strings.Contains(result, "\x1b[") {
		t.Errorf("FormatSummaryCard() with NO_COLOR=1 contains ANSI codes:\n%s", result)
	}

	// Should still contain text content
	if !strings.Contains(result, "ANALYSIS RESULTS") {
		t.Errorf("FormatSummaryCard() with NO_COLOR=1 missing 'ANALYSIS RESULTS'")
	}
	if !strings.Contains(result, "SAFE TO DEPLOY") {
		t.Errorf("FormatSummaryCard() with NO_COLOR=1 missing 'SAFE TO DEPLOY'")
	}
}

func TestFormatOperationCardNoColor(t *testing.T) {
	origNoColor := os.Getenv("NO_COLOR")
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("NO_COLOR", origNoColor)
		os.Setenv("TERM", origTerm)
	}()

	os.Setenv("NO_COLOR", "1")
	os.Setenv("TERM", "xterm-256color")

	op := &models.Operation{
		Type:                 models.OperationTypeAlterTable,
		TableName:            "users",
		SQL:                  "ALTER TABLE users ADD COLUMN phone VARCHAR(20);",
		RiskScore:            15,
		LockType:             models.LockTypeAccessExclusive,
		LockDurationMS:       5,
		EstimatedTimeSeconds: 0.5,
		BackwardCompatible:   true,
		RequiresRewrite:      false,
		Recommendations:      []string{"Consider using IF NOT EXISTS"},
	}

	result := FormatOperationCard(op, 1)

	// With NO_COLOR=1, output should have NO ANSI escape codes
	if strings.Contains(result, "\x1b[") {
		t.Errorf("FormatOperationCard() with NO_COLOR=1 contains ANSI codes:\n%s", result)
	}

	// Should still contain text content
	if !strings.Contains(result, "Lock Analysis") {
		t.Errorf("FormatOperationCard() with NO_COLOR=1 missing 'Lock Analysis'")
	}
}

func TestFormatSummaryCardNoEmoji(t *testing.T) {
	origNoEmoji := os.Getenv("TAPA_NO_EMOJI")
	defer os.Setenv("TAPA_NO_EMOJI", origNoEmoji)

	os.Setenv("TAPA_NO_EMOJI", "1")

	migration := models.NewMigration("test.sql")
	migration.AddOperation(&models.Operation{
		RiskScore:          15,
		BackwardCompatible: true,
	})

	result := FormatSummaryCard(migration)

	// With TAPA_NO_EMOJI=1, should use text fallbacks
	if strings.Contains(result, "🟢") || strings.Contains(result, "✨") {
		t.Errorf("FormatSummaryCard() with TAPA_NO_EMOJI=1 still contains emojis:\n%s", result)
	}
	// Should contain fallback characters
	if !strings.Contains(result, "[+]") && !strings.Contains(result, "*") {
		t.Errorf("FormatSummaryCard() with TAPA_NO_EMOJI=1 missing emoji fallbacks")
	}
}

func TestFormatExecutionTimeMilliseconds(t *testing.T) {
	// Save and restore env
	origNoColor := os.Getenv("NO_COLOR")
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("NO_COLOR", origNoColor)
		os.Setenv("TERM", origTerm)
	}()
	os.Setenv("NO_COLOR", "1")
	os.Setenv("TERM", "xterm-256color")

	tests := []struct {
		name    string
		seconds float64
		want    string
	}{
		{"very fast", 0.005, "< 10ms"},
		{"50ms", 0.05, "50ms"},
		{"100ms", 0.1, "100ms"},
		{"200ms", 0.2, "200ms"},
		{"500ms", 0.5, "500ms"},
		{"1 second", 1.0, "1.0 seconds"},
		{"5 seconds", 5.0, "5.0 seconds"},
		{"2 minutes", 120.0, "2.0 minutes"},
		{"10 minutes", 600.0, "10.0 minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatExecutionTime(tt.seconds)
			if got != tt.want {
				t.Errorf("formatExecutionTime(%v) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestFormatOperationCardSQLTruncation(t *testing.T) {
	longSQL := "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT 'test@example.com' " +
		"CONSTRAINT email_check CHECK (email LIKE '%@%') CONSTRAINT email_unique UNIQUE (email) " +
		"WITH (fillfactor=80) TABLESPACE fast_storage;"

	op := &models.Operation{
		Type:                 models.OperationTypeAddColumn,
		TableName:            "users",
		SQL:                  longSQL,
		RiskScore:            15,
		LockType:             models.LockTypeAccessExclusive,
		LockDurationMS:       100,
		EstimatedTimeSeconds: 0.1,
		BackwardCompatible:   true,
		RequiresRewrite:      false,
	}

	result := FormatOperationCard(op, 1)

	// Should contain the SQL start
	if !strings.Contains(result, "SQL:") {
		t.Errorf("FormatOperationCard() missing SQL prefix")
	}

	// Should contain ellipsis for truncated SQL
	if !strings.Contains(result, "...") {
		t.Errorf("FormatOperationCard() should truncate long SQL with '...'")
	}
}
