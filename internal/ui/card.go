package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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
		titleVisLen := visibleWidth(titlePadded)
		remainingWidth := width - titleVisLen - 2
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

// visibleWidth returns the visible character width of a string,
// ignoring ANSI escape codes and accounting for wide characters (emojis).
func visibleWidth(s string) int {
	stripped := ansiRegex.ReplaceAllString(s, "")
	width := 0
	for _, r := range stripped {
		if isWideRune(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

// isWideRune returns true if the rune is a wide character (takes 2 columns in terminal).
// This covers emojis and other characters that occupy 2 cells.
func isWideRune(r rune) bool {
	// Emoji and symbol ranges that are typically displayed as 2 columns wide
	// Miscellaneous Symbols and Pictographs, Emoticons, etc.
	if r >= 0x1F300 && r <= 0x1FAF8 {
		return true
	}
	// Specific wide dingbats/symbols used in our output
	switch r {
	case 0x2728: // ✨
		return true
	case 0x26A0: // ⚠ - varies by terminal, treat as wide for safety
		return true
	}
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// Fullwidth forms
	if r >= 0xFF01 && r <= 0xFF60 {
		return true
	}
	return false
}

// padOrTruncate pads or truncates a string to the specified visible width.
// Handles ANSI color codes and multi-byte UTF-8 correctly.
func padOrTruncate(s string, width int) string {
	vis := visibleWidth(s)

	if vis >= width {
		return truncateWithAnsi(s, width)
	}

	padding := width - vis
	return s + strings.Repeat(" ", padding)
}

// truncateWithAnsi truncates a string to the specified visible width, preserving ANSI codes
func truncateWithAnsi(s string, width int) string {
	visibleCount := 0
	var result strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); {
		// Check if this is the start of an ANSI code
		remaining := string(runes[i:])
		if runes[i] == '\x1b' {
			match := ansiRegex.FindStringIndex(remaining)
			if match != nil && match[0] == 0 {
				result.WriteString(remaining[:match[1]])
				i += utf8.RuneCountInString(remaining[:match[1]])
				continue
			}
		}

		runeWidth := 1
		if isWideRune(runes[i]) {
			runeWidth = 2
		}
		if visibleCount+runeWidth > width {
			break
		}
		result.WriteRune(runes[i])
		visibleCount += runeWidth
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

// FormatSummaryCard creates a summary card showing aggregate migration stats.
// Per-operation details (lock, time) belong in FormatOperationCard instead.
func FormatSummaryCard(migration *models.Migration) string {
	summary := calculateSummary(migration)
	width := 60

	riskColor := getRiskColor(summary.MaxRiskScore)
	riskLevel := getRiskLevelFromScore(summary.MaxRiskScore)

	var lines []string

	// ── Risk Score ──
	lines = append(lines, "\x1b[1mRisk Score\x1b[0m")
	progressBar := DrawVisualProgressBar(summary.MaxRiskScore, 100, 28, riskColor)
	lines = append(lines, fmt.Sprintf("%s  %s%d/100\x1b[0m", progressBar, riskColor, summary.MaxRiskScore))
	lines = append(lines, fmt.Sprintf("Status: %s%s\x1b[0m %s", riskColor, riskLevel, getStatusIcon(summary.MaxRiskScore)))
	lines = append(lines, "")

	// ── Est. Time ──
	lines = append(lines, fmt.Sprintf("\x1b[1mEst. Time:\x1b[0m %s", formatDuration(summary.TotalTime)))
	lines = append(lines, "")

	// ── Risk Breakdown ──
	lines = append(lines, "\x1b[1mRisk Breakdown:\x1b[0m")
	lines = append(lines, formatRiskBreakdown("  ├── Low     ", summary.LowRisk, "\x1b[32m"))
	lines = append(lines, formatRiskBreakdown("  ├── Medium  ", summary.MediumRisk, "\x1b[34m"))
	lines = append(lines, formatRiskBreakdown("  ├── High    ", summary.HighRisk, "\x1b[33m"))
	lines = append(lines, formatRiskBreakdown("  └── Critical", summary.CriticalRisk, "\x1b[31m"))
	lines = append(lines, "")

	// ── Compatibility ──
	lines = append(lines, "\x1b[1mCompatibility:\x1b[0m")
	allCompat := summary.BackwardCompatible == summary.TotalOps
	if allCompat {
		lines = append(lines, fmt.Sprintf("  \x1b[32m✓\x1b[0m All operations backward compatible (%d/%d)", summary.BackwardCompatible, summary.TotalOps))
	} else {
		lines = append(lines, fmt.Sprintf("  \x1b[33m⚠\x1b[0m %d/%d operations backward compatible", summary.BackwardCompatible, summary.TotalOps))
	}
	hasBreaking := checkForBreakingChanges(migration.Operations)
	if !hasBreaking {
		lines = append(lines, "  \x1b[32m✓\x1b[0m No breaking changes")
	} else {
		lines = append(lines, "  \x1b[31m✗\x1b[0m Contains breaking changes")
	}
	rollingSafe := isRollingSafe(summary.MaxRiskScore, allCompat)
	if rollingSafe {
		lines = append(lines, "  \x1b[32m✓\x1b[0m Rolling deployment safe")
	} else {
		lines = append(lines, "  \x1b[33m⚠\x1b[0m Requires maintenance window")
	}

	// Build the two-box layout
	mainCard := DrawBox("ANALYSIS RESULTS", lines, width)
	bottomLines := getBottomMessage(summary.MaxRiskScore)
	bottomCard := DrawBox("", bottomLines, width)

	return mainCard + "\n" + bottomCard
}

// wrapText wraps text to fit within the given visible width.
// Each continuation line is indented by the given prefix.
func wrapText(text string, maxWidth int, continuationPrefix string) []string {
	if visibleWidth(text) <= maxWidth {
		return []string{text}
	}

	var result []string
	words := strings.Fields(stripAnsi(text))
	line := ""
	for _, word := range words {
		candidate := line
		if candidate != "" {
			candidate += " "
		}
		candidate += word

		if visibleWidth(candidate) > maxWidth && line != "" {
			result = append(result, line)
			line = continuationPrefix + word
		} else {
			line = candidate
		}
	}
	if line != "" {
		result = append(result, line)
	}
	return result
}

// FormatOperationCard creates a bordered card for a single operation
func FormatOperationCard(op *models.Operation, index int) string {
	width := 60
	innerWidth := width - 4 // account for │ + space on each side
	riskColor := getRiskColor(op.RiskScore)

	var lines []string

	// Header: SQL statement (word-wrapped)
	sqlPrefix := "SQL: "
	sqlText := sqlPrefix + op.SQL
	sqlLines := wrapText(sqlText, innerWidth, "     ")
	for _, sl := range sqlLines {
		lines = append(lines, sl)
	}
	lines = append(lines, "")

	// ── Risk Score ──
	lines = append(lines, "\x1b[1mRisk Score\x1b[0m")
	bar := DrawVisualProgressBar(op.RiskScore, 100, 28, riskColor)
	lines = append(lines, fmt.Sprintf("%s  %s%d/100\x1b[0m", bar, riskColor, op.RiskScore))
	riskLevel := getRiskLevelFromScore(op.RiskScore)
	lines = append(lines, fmt.Sprintf("Status: %s%s\x1b[0m %s", riskColor, riskLevel, getStatusIcon(op.RiskScore)))
	lines = append(lines, "")

	// ── Lock Analysis ──
	lines = append(lines, "\x1b[1mLock Analysis\x1b[0m")
	lockColor := getLockTypeColor(op.LockType)
	lockDesc := getLockDescription(op.LockType)
	lines = append(lines, fmt.Sprintf("  ├── Type      %s%s\x1b[0m %s", lockColor, op.LockType, lockDesc))
	durationStr := formatLockDuration(op.LockDurationMS)
	lines = append(lines, fmt.Sprintf("  ├── Duration  %s", durationStr))
	queriesStr := estimateQueriesAffected(op.LockType)
	lines = append(lines, fmt.Sprintf("  └── Queries   %s", queriesStr))
	lines = append(lines, "")

	// ── Time Estimate ──
	lines = append(lines, "\x1b[1mTime Estimate\x1b[0m")
	execTime := formatExecutionTime(op.EstimatedTimeSeconds)
	lines = append(lines, fmt.Sprintf("  ├── Execution  %s", execTime))
	tableSizeInfo := formatTableSizeInfo(op)
	lines = append(lines, fmt.Sprintf("  └── Table Size %s", tableSizeInfo))
	lines = append(lines, "")

	// ── Compatibility ──
	lines = append(lines, "\x1b[1mCompatibility\x1b[0m")
	if op.BackwardCompatible {
		lines = append(lines, "  \x1b[32m✓\x1b[0m Backward compatible")
	} else {
		lines = append(lines, "  \x1b[33m⚠\x1b[0m May break backward compatibility")
	}
	if !op.RequiresRewrite {
		lines = append(lines, "  \x1b[32m✓\x1b[0m No table rewrite")
	} else {
		lines = append(lines, "  \x1b[31m✗\x1b[0m Requires full table rewrite")
	}

	// ── Recommendations ──
	if len(op.Recommendations) > 0 {
		lines = append(lines, "")
		lines = append(lines, "\x1b[1mRecommendations\x1b[0m")
		for _, rec := range op.Recommendations {
			wrapped := wrapText("  • "+rec, innerWidth, "    ")
			lines = append(lines, wrapped...)
		}
	}

	title := fmt.Sprintf("%s on %s", op.Type, op.TableName)
	return DrawBox(title, lines, width)
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

// getBottomMessage returns appropriate message lines based on risk score
func getBottomMessage(maxRiskScore int) []string {
	switch {
	case maxRiskScore >= 76:
		return []string{"  \x1b[31m⚠  CRITICAL: Review carefully before proceeding\x1b[0m"}
	case maxRiskScore >= 51:
		return []string{"  \x1b[33m⚠  Requires thorough testing in staging environment\x1b[0m"}
	case maxRiskScore >= 26:
		return []string{"  \x1b[34mℹ  Standard precautions recommended\x1b[0m"}
	default:
		return []string{
			"  \x1b[32m✨ SAFE TO DEPLOY\x1b[0m",
			"  \x1b[32mThis migration can run without downtime.\x1b[0m",
		}
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
