package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/iamsr/tapa/pkg/models"
)

// Box drawing characters
const (
	boxTopLeft     = "╭"
	boxTopRight    = "╮"
	boxBottomLeft  = "╰"
	boxBottomRight = "╯"
	boxHorizontal  = "─"
	boxVertical    = "│"
)

// ansiRegex is compiled once at package initialization for performance
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// DrawBox draws a box with the given content lines
func DrawBox(title string, lines []string, width int) string {
	// Validate width - minimum 4 chars for borders + padding
	if width < 4 {
		width = 4
	}

	// Handle empty or nil lines
	if len(lines) == 0 {
		lines = []string{" "}
	}

	var result strings.Builder

	// Top border with title
	result.WriteString(boxTopLeft)
	if title != "" {
		titlePadded := " " + title + " "
		titleLen := len(titlePadded)
		remainingWidth := width - titleLen - 2
		if remainingWidth < 0 {
			remainingWidth = 0
		}
		result.WriteString(strings.Repeat(boxHorizontal, 1))
		result.WriteString(titlePadded)
		result.WriteString(strings.Repeat(boxHorizontal, remainingWidth))
	} else {
		result.WriteString(strings.Repeat(boxHorizontal, width-2))
	}
	result.WriteString(boxTopRight)
	result.WriteString("\n")

	// Content lines
	for _, line := range lines {
		result.WriteString(boxVertical)
		result.WriteString(" ")
		result.WriteString(padOrTruncate(line, width-4))
		result.WriteString(" ")
		result.WriteString(boxVertical)
		result.WriteString("\n")
	}

	// Bottom border
	result.WriteString(boxBottomLeft)
	result.WriteString(strings.Repeat(boxHorizontal, width-2))
	result.WriteString(boxBottomRight)

	return result.String()
}

// padOrTruncate pads or truncates a string to the specified width
// Handles ANSI color codes correctly
func padOrTruncate(s string, width int) string {
	// Strip ANSI codes to calculate visible length
	visibleLen := len(stripAnsi(s))

	if visibleLen >= width {
		// Truncate - need to be careful with ANSI codes
		return truncateWithAnsi(s, width)
	}

	// Pad with spaces
	padding := width - visibleLen
	return s + strings.Repeat(" ", padding)
}

// truncateWithAnsi truncates a string to the specified width, preserving ANSI codes
func truncateWithAnsi(s string, width int) string {
	visibleCount := 0
	var result strings.Builder

	for i := 0; i < len(s); {
		// Check if this is the start of an ANSI code
		if s[i] == '\x1b' {
			// Find the end of the ANSI code
			match := ansiRegex.FindStringIndex(s[i:])
			if match != nil && match[0] == 0 {
				// Include the ANSI code without counting it
				result.WriteString(s[i : i+match[1]])
				i += match[1]
				continue
			}
		}

		// Regular character
		if visibleCount >= width {
			break
		}
		result.WriteByte(s[i])
		visibleCount++
		i++
	}

	return result.String()
}

// stripAnsi removes ANSI color codes from a string
func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// MigrationSummary contains summary statistics for a migration
type MigrationSummary struct {
	TotalOps           int
	LowRisk            int
	MediumRisk         int
	HighRisk           int
	CriticalRisk       int
	BackwardCompatible int
	TotalTime          time.Duration
	MaxRiskScore       int
}

// FormatSummaryCard creates a beautiful summary card for a migration
func FormatSummaryCard(migration *models.Migration) string {
	summary := calculateSummary(migration)
	var lines []string

	// Status line with emoji
	statusIcon := getStatusIcon(summary.MaxRiskScore)
	riskLevel := getRiskLevelFromScore(summary.MaxRiskScore)
	riskColor := getRiskColor(summary.MaxRiskScore)
	statusLine := fmt.Sprintf("%s Status: %s%s\x1b[0m", statusIcon, riskColor, riskLevel)
	lines = append(lines, statusLine)

	// Empty line
	lines = append(lines, "")

	// Overall progress bar
	completedOps := 0 // For summary card, we show total operations as "to be done"
	progressBar := DrawVisualProgressBar(completedOps, summary.TotalOps, 30, "\x1b[32m")
	lines = append(lines, fmt.Sprintf("Progress: %s %d/%d ops", progressBar, completedOps, summary.TotalOps))

	// Estimated time
	lines = append(lines, fmt.Sprintf("Est. Time: %s", formatDuration(summary.TotalTime)))

	// Empty line
	lines = append(lines, "")

	// Risk breakdown (tree-style)
	lines = append(lines, "Risk Breakdown:")
	lines = append(lines, formatRiskBreakdown("├── Low", summary.LowRisk, "\x1b[32m"))
	lines = append(lines, formatRiskBreakdown("├── Medium", summary.MediumRisk, "\x1b[34m"))
	lines = append(lines, formatRiskBreakdown("├── High", summary.HighRisk, "\x1b[33m"))
	lines = append(lines, formatRiskBreakdown("└── Critical", summary.CriticalRisk, "\x1b[31m"))

	// Empty line
	lines = append(lines, "")

	// Compatibility checklist
	lines = append(lines, "Compatibility:")
	compatLine := formatCompatibilityCheck(summary.BackwardCompatible, summary.TotalOps)
	lines = append(lines, compatLine)

	// Bottom message
	bottomMsg := getBottomMessage(summary.MaxRiskScore)
	lines = append(lines, "")
	lines = append(lines, bottomMsg)

	return DrawBox("📊 Migration Summary", lines, 70)
}

// calculateSummary computes summary statistics from a migration
func calculateSummary(migration *models.Migration) MigrationSummary {
	summary := MigrationSummary{}
	summary.TotalOps = len(migration.Operations)

	for _, op := range migration.Operations {
		// Count risk levels
		switch op.RiskLevel() {
		case models.RiskLevelLow:
			summary.LowRisk++
		case models.RiskLevelMedium:
			summary.MediumRisk++
		case models.RiskLevelHigh:
			summary.HighRisk++
		case models.RiskLevelCritical:
			summary.CriticalRisk++
		}

		// Track backward compatibility
		if op.BackwardCompatible {
			summary.BackwardCompatible++
		}

		// Sum total time
		summary.TotalTime += time.Duration(op.EstimatedTimeSeconds * float64(time.Second))

		// Track max risk score
		if op.RiskScore > summary.MaxRiskScore {
			summary.MaxRiskScore = op.RiskScore
		}
	}

	return summary
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// formatRiskBreakdown formats a risk breakdown line
func formatRiskBreakdown(label string, count int, color string) string {
	bar := ""
	barWidth := 20
	if count > 0 {
		// Simple bar representation
		filledWidth := count
		if filledWidth > barWidth {
			filledWidth = barWidth
		}
		bar = strings.Repeat("▪", filledWidth)
	}
	if color != "" && bar != "" {
		bar = color + bar + "\x1b[0m"
	}
	return fmt.Sprintf("  %s %s %d", label, bar, count)
}

// formatCompatibilityCheck formats the compatibility check line
func formatCompatibilityCheck(compatible int, total int) string {
	if compatible == total {
		return fmt.Sprintf("  ✓ All operations backward compatible (%d/%d)", compatible, total)
	}
	return fmt.Sprintf("  ⚠ %d/%d operations backward compatible", compatible, total)
}

// getBottomMessage returns an appropriate message based on risk score
func getBottomMessage(maxRiskScore int) string {
	switch {
	case maxRiskScore >= 76:
		return "⚠️  CRITICAL: Review carefully before proceeding"
	case maxRiskScore >= 51:
		return "⚠️  HIGH RISK: Test thoroughly in staging"
	case maxRiskScore >= 26:
		return "ℹ️  MEDIUM RISK: Standard precautions apply"
	default:
		return "✓ LOW RISK: Safe to proceed"
	}
}

// getStatusIcon returns an emoji icon based on risk score
func getStatusIcon(riskScore int) string {
	switch {
	case riskScore >= 76:
		return "🔴"
	case riskScore >= 51:
		return "🟠"
	case riskScore >= 26:
		return "🟡"
	default:
		return "🟢"
	}
}

// getRiskColor returns ANSI color code for risk score
func getRiskColor(riskScore int) string {
	switch {
	case riskScore >= 76:
		return "\x1b[31m" // Red
	case riskScore >= 51:
		return "\x1b[33m" // Yellow
	case riskScore >= 26:
		return "\x1b[34m" // Blue
	default:
		return "\x1b[32m" // Green
	}
}

// getRiskLevelFromScore returns risk level string from score
func getRiskLevelFromScore(riskScore int) string {
	switch {
	case riskScore >= 76:
		return "CRITICAL"
	case riskScore >= 51:
		return "HIGH"
	case riskScore >= 26:
		return "MEDIUM"
	default:
		return "LOW"
	}
}
