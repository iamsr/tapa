package ui

import (
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
