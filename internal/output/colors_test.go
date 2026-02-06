package output

import (
	"os"
	"testing"

	"github.com/iamsr/tapa/pkg/models"
)

func TestColorsEnabled(t *testing.T) {
	// Save original env vars
	origNoColor := os.Getenv("NO_COLOR")
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("NO_COLOR", origNoColor)
		os.Setenv("TERM", origTerm)
	}()

	tests := []struct {
		name     string
		noColor  string
		term     string
		expected bool
	}{
		{"NO_COLOR set", "1", "xterm", false},
		{"TERM is dumb", "", "dumb", false},
		{"TERM is empty", "", "", false},
		{"Normal terminal", "", "xterm-256color", false}, // Will be false due to stdout not being a terminal in tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("NO_COLOR", tt.noColor)
			os.Setenv("TERM", tt.term)

			result := ColorsEnabled()
			if result != tt.expected {
				t.Errorf("ColorsEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestColorize(t *testing.T) {
	// Save and restore NO_COLOR
	origNoColor := os.Getenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", origNoColor)

	os.Setenv("NO_COLOR", "1") // Force colors off for testing

	tests := []struct {
		name  string
		text  string
		color string
		want  string
	}{
		{"Red text", "error", colorRed, "error"},
		{"Green text", "success", colorGreen, "success"},
		{"Yellow text", "warning", colorYellow, "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Colorize(tt.text, tt.color)
			if got != tt.want {
				t.Errorf("Colorize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRiskColor(t *testing.T) {
	tests := []struct {
		name      string
		riskScore int
		want      string
	}{
		{"Low risk", 10, colorGreen},
		{"Medium-low risk", 25, colorBlue},
		{"Medium risk", 50, colorYellow},
		{"High risk", 75, colorRed},
		{"Critical risk", 95, colorRed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RiskColor(tt.riskScore)
			if got != tt.want {
				t.Errorf("RiskColor(%d) = %v, want %v", tt.riskScore, got, tt.want)
			}
		})
	}
}

func TestLockTypeColor(t *testing.T) {
	tests := []struct {
		name     string
		lockType models.LockType
		want     string
	}{
		{"None", models.LockTypeNone, colorGreen},
		{"Row Exclusive", models.LockTypeRowExclusive, colorGreen},
		{"Share", models.LockTypeShare, colorYellow},
		{"Share Update Exclusive", models.LockTypeShareUpdateExclusive, colorYellow},
		{"Exclusive", models.LockTypeExclusive, colorRed},
		{"Access Exclusive", models.LockTypeAccessExclusive, colorRed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LockTypeColor(tt.lockType)
			if got != tt.want {
				t.Errorf("LockTypeColor(%v) = %v, want %v", tt.lockType, got, tt.want)
			}
		})
	}
}
