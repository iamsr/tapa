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

	// Get the first operation for detailed lock/time info
	var mainOp *models.Operation
	if len(migration.Operations) > 0 {
		mainOp = migration.Operations[0]
	}

	riskColor := getRiskColor(summary.MaxRiskScore)
	riskLevel := getRiskLevelFromScore(summary.MaxRiskScore)

	// ===== RISK SCORE SECTION =====
	lines = append(lines, "\x1b[1mRisk Score\x1b[0m")
	progressBar := DrawVisualProgressBar(summary.MaxRiskScore, 100, 28, riskColor)
	lines = append(lines, fmt.Sprintf("%s  %s%d/100\x1b[0m", progressBar, riskColor, summary.MaxRiskScore))
	statusLine := fmt.Sprintf("Status: %s%s ✓\x1b[0m", riskColor, riskLevel)
	lines = append(lines, statusLine)
	lines = append(lines, "")

	// ===== LOCK ANALYSIS SECTION =====
	lines = append(lines, "\x1b[1mLock Analysis\x1b[0m")
	if mainOp != nil {
		lockTypeColor := getLockTypeColor(mainOp.LockType)
		lockTypeFormatted := fmt.Sprintf("%s%s\x1b[0m", lockTypeColor, mainOp.LockType)

		// Add description based on lock type
		lockDesc := getLockDescription(mainOp.LockType)
		lines = append(lines, fmt.Sprintf("├── Type      %s %s", lockTypeFormatted, lockDesc))

		// Format duration
		durationStr := formatLockDuration(mainOp.LockDurationMS)
		lines = append(lines, fmt.Sprintf("├── Duration  %s", durationStr))

		// Queries affected (estimate based on lock type)
		queriesAffected := estimateQueriesAffected(mainOp.LockType)
		lines = append(lines, fmt.Sprintf("└── Queries   %s", queriesAffected))
	} else {
		lines = append(lines, "└── No operations to analyze")
	}
	lines = append(lines, "")

	// ===== TIME ESTIMATE SECTION =====
	lines = append(lines, "\x1b[1mTime Estimate\x1b[0m")
	if mainOp != nil {
		execTime := formatExecutionTime(mainOp.EstimatedTimeSeconds)
		lines = append(lines, fmt.Sprintf("├── Execution  %s", execTime))

		// Table size info (if available, otherwise show generic message)
		tableSizeInfo := formatTableSizeInfo(mainOp)
		lines = append(lines, fmt.Sprintf("└── Table Size %s", tableSizeInfo))
	} else {
		lines = append(lines, "└── No time estimate available")
	}
	lines = append(lines, "")

	// ===== COMPATIBILITY CHECK SECTION =====
	lines = append(lines, "\x1b[1mCompatibility Check\x1b[0m")

	// Check backward compatibility
	allBackwardCompatible := summary.BackwardCompatible == summary.TotalOps
	if allBackwardCompatible {
		lines = append(lines, "\x1b[32m✓ Backward compatible\x1b[0m")
	} else {
		lines = append(lines, "\x1b[33m⚠ May break backward compatibility\x1b[0m")
	}

	// Check for breaking changes (based on operation types)
	hasBreakingChanges := checkForBreakingChanges(migration.Operations)
	if !hasBreakingChanges {
		lines = append(lines, "\x1b[32m✓ No breaking changes\x1b[0m")
	} else {
		lines = append(lines, "\x1b[33m⚠ Contains breaking changes\x1b[0m")
	}

	// Check rolling deployment safety
	rollingSafe := isRollingSafe(summary.MaxRiskScore, allBackwardCompatible)
	if rollingSafe {
		lines = append(lines, "\x1b[32m✓ Rolling deployment safe\x1b[0m")
	} else {
		lines = append(lines, "\x1b[33m⚠ Requires maintenance window\x1b[0m")
	}

	// Create main card
	mainCard := DrawBox("ANALYSIS RESULTS", lines, 65)

	// ===== BOTTOM MESSAGE BOX =====
	bottomLines := []string{getBottomMessage(summary.MaxRiskScore)}
	bottomCard := DrawBox("", bottomLines, 65)

	return mainCard + "\n" + bottomCard
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
		return "  \x1b[31m⚠  CRITICAL: Review carefully before proceeding\x1b[0m"
	case maxRiskScore >= 51:
		return "  \x1b[33m⚠  Requires thorough testing in staging environment\x1b[0m"
	case maxRiskScore >= 26:
		return "  \x1b[34mℹ  Standard precautions recommended\x1b[0m"
	default:
		return "  \x1b[32m✨ SAFE TO DEPLOY\x1b[0m\n  \x1b[32mThis migration can run without downtime.\x1b[0m"
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
		return "CRITICAL RISK"
	case riskScore >= 51:
		return "HIGH RISK"
	case riskScore >= 26:
		return "MEDIUM RISK"
	default:
		return "LOW RISK"
	}
}

// getLockTypeColor returns ANSI color code for lock type
func getLockTypeColor(lockType models.LockType) string {
	switch lockType {
	case models.LockTypeAccessExclusive:
		return "\x1b[31m" // Red
	case models.LockTypeExclusive:
		return "\x1b[33m" // Yellow
	case models.LockTypeShare, models.LockTypeShareUpdateExclusive:
		return "\x1b[34m" // Blue
	default:
		return "\x1b[32m" // Green
	}
}

// getLockDescription returns a human-readable description of the lock type
func getLockDescription(lockType models.LockType) string {
	switch lockType {
	case models.LockTypeAccessExclusive:
		return "(blocks all access)"
	case models.LockTypeExclusive:
		return "(blocks concurrent DDL)"
	case models.LockTypeShare:
		return "(allows reads)"
	case models.LockTypeShareUpdateExclusive:
		return "(allows reads, blocks writes)"
	case models.LockTypeRowExclusive:
		return "(minimal locking)"
	default:
		return "(metadata only)"
	}
}

// formatLockDuration formats lock duration in a human-readable way
func formatLockDuration(durationMS int64) string {
	if durationMS < 10 {
		return "\x1b[32m< 10ms\x1b[0m"
	} else if durationMS < 100 {
		return fmt.Sprintf("\x1b[32m%dms\x1b[0m", durationMS)
	} else if durationMS < 1000 {
		return fmt.Sprintf("\x1b[33m%dms\x1b[0m", durationMS)
	} else {
		seconds := float64(durationMS) / 1000.0
		return fmt.Sprintf("\x1b[31m%.1fs\x1b[0m", seconds)
	}
}

// estimateQueriesAffected estimates number of queries affected by lock
func estimateQueriesAffected(lockType models.LockType) string {
	switch lockType {
	case models.LockTypeAccessExclusive:
		return "\x1b[31mAll queries blocked\x1b[0m"
	case models.LockTypeExclusive:
		return "\x1b[33mDDL queries blocked\x1b[0m"
	case models.LockTypeShare:
		return "\x1b[34mWrites blocked\x1b[0m"
	case models.LockTypeShareUpdateExclusive:
		return "\x1b[34mWrites delayed\x1b[0m"
	default:
		return "\x1b[32m0 affected\x1b[0m"
	}
}

// formatExecutionTime formats execution time in a human-readable way
func formatExecutionTime(seconds float64) string {
	if seconds < 1 {
		return "\x1b[32m< 1 second\x1b[0m"
	} else if seconds < 60 {
		return fmt.Sprintf("\x1b[32m%.1f seconds\x1b[0m", seconds)
	} else if seconds < 300 { // < 5 minutes
		minutes := seconds / 60.0
		return fmt.Sprintf("\x1b[33m%.1f minutes\x1b[0m", minutes)
	} else {
		minutes := seconds / 60.0
		return fmt.Sprintf("\x1b[31m%.1f minutes\x1b[0m", minutes)
	}
}

// formatTableSizeInfo formats table size information
func formatTableSizeInfo(op *models.Operation) string {
	if op.RequiresRewrite {
		return "\x1b[31m(full table rewrite needed)\x1b[0m"
	}
	// Generic message since we don't have actual row count in the operation struct
	return "\x1b[32m(no rewrite needed)\x1b[0m"
}

// checkForBreakingChanges checks if operations contain breaking changes
func checkForBreakingChanges(operations []*models.Operation) bool {
	for _, op := range operations {
		switch op.Type {
		case models.OperationTypeDropColumn, models.OperationTypeDropTable, models.OperationTypeDropIndex:
			return true
		case models.OperationTypeAlterColumn:
			// Type changes or NOT NULL additions are breaking
			if strings.Contains(strings.ToUpper(op.SQL), "TYPE") ||
				strings.Contains(strings.ToUpper(op.SQL), "NOT NULL") {
				return true
			}
		}
	}
	return false
}

// isRollingSafe determines if migration is safe for rolling deployment
func isRollingSafe(maxRiskScore int, allBackwardCompatible bool) bool {
	return maxRiskScore < 51 && allBackwardCompatible
}
