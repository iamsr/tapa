package output

import (
	"os"

	"github.com/iamsr/tapa/pkg/models"
)

// ANSI color codes
var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorBold   = "\033[1m"
)

// ColorsEnabled checks if color output should be enabled
func ColorsEnabled() bool {
	// Respect NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if TERM is set and not dumb
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}

	// Check if stdout is a terminal
	// Note: This is a simple check - could be enhanced with terminal detection
	fileInfo, _ := os.Stdout.Stat()
	if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	return true
}

// Colorize wraps text in the specified color code
func Colorize(text string, color string) string {
	if !ColorsEnabled() {
		return text
	}
	return color + text + colorReset
}

// Red returns text colored in red
func Red(text string) string {
	return Colorize(text, colorRed)
}

// Green returns text colored in green
func Green(text string) string {
	return Colorize(text, colorGreen)
}

// Yellow returns text colored in yellow
func Yellow(text string) string {
	return Colorize(text, colorYellow)
}

// Blue returns text colored in blue
func Blue(text string) string {
	return Colorize(text, colorBlue)
}

// Bold returns text in bold
func Bold(text string) string {
	return Colorize(text, colorBold)
}

// RiskColor returns the appropriate color for a risk score
func RiskColor(riskScore int) string {
	switch {
	case riskScore >= 75:
		return colorRed
	case riskScore >= 50:
		return colorYellow
	case riskScore >= 25:
		return colorBlue
	default:
		return colorGreen
	}
}

// LockTypeColor returns the appropriate color for a lock type
func LockTypeColor(lockType models.LockType) string {
	switch lockType {
	case models.LockTypeAccessExclusive, models.LockTypeExclusive:
		return colorRed
	case models.LockTypeShareUpdateExclusive, models.LockTypeShare:
		return colorYellow
	default:
		return colorGreen
	}
}
