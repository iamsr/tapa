package ui

import (
	"fmt"
	"os"
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

// ANSI color codes used within the ui package
const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
	ansiBold   = "\x1b[1m"
)

// ansiRegex is compiled once at package initialization for performance
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// colorsEnabled checks if color output should be enabled.
// Mirrors output.ColorsEnabled() to avoid circular dependency.
func colorsEnabled() bool {
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
	fileInfo, _ := os.Stdout.Stat()
	if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	return true
}

// colorize wraps text in an ANSI color code, respecting NO_COLOR.
func colorize(text, color string) string {
	if !colorsEnabled() {
		return text
	}
	return color + text + ansiReset
}

// bold wraps text in ANSI bold, respecting NO_COLOR.
func bold(text string) string {
	return colorize(text, ansiBold)
}

// emojisEnabled checks if emoji output should be enabled.
// Set TAPA_NO_EMOJI=1 to disable emojis (useful for terminals with poor emoji support).
func emojisEnabled() bool {
	return os.Getenv("TAPA_NO_EMOJI") == ""
}

// emoji returns the emoji string if emojis are enabled, otherwise the fallback text.
func emoji(emojiStr, fallback string) string {
	if emojisEnabled() {
		return emojiStr
	}
	return fallback
}

// StepCheck returns a green checkmark for step-by-step progress output.
func StepCheck() string {
	return colorize(emoji("✓", "[ok]"), ansiGreen)
}

// StepWarn returns a yellow warning symbol for step-by-step progress output.
func StepWarn() string {
	return colorize(emoji("⚠", "[!]"), ansiYellow)
}

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
	width := 76

	riskColor := getRiskColor(summary.MaxRiskScore)
	riskLevel := getRiskLevelFromScore(summary.MaxRiskScore)

	var lines []string

	// ── Risk Score ──
	lines = append(lines, bold("Risk Score"))
	progressBar := DrawVisualProgressBar(summary.MaxRiskScore, 100, 28, riskColor)
	lines = append(lines, fmt.Sprintf("%s  %s", progressBar, colorize(fmt.Sprintf("%d/100", summary.MaxRiskScore), riskColor)))
	lines = append(lines, fmt.Sprintf("Status: %s %s", colorize(riskLevel, riskColor), getStatusIcon(summary.MaxRiskScore)))
	lines = append(lines, "")

	// ── Est. Time ──
	lines = append(lines, fmt.Sprintf("%s %s", bold("Est. Time:"), formatDuration(summary.TotalTime)))
	lines = append(lines, "")

	// ── Risk Breakdown ──
	lines = append(lines, bold("Risk Breakdown:"))
	lines = append(lines, formatRiskBreakdown("  ├── Low     ", summary.LowRisk, ansiGreen))
	lines = append(lines, formatRiskBreakdown("  ├── Medium  ", summary.MediumRisk, ansiBlue))
	lines = append(lines, formatRiskBreakdown("  ├── High    ", summary.HighRisk, ansiYellow))
	lines = append(lines, formatRiskBreakdown("  └── Critical", summary.CriticalRisk, ansiRed))
	lines = append(lines, "")

	// ── Compatibility ──
	lines = append(lines, bold("Compatibility:"))
	allCompat := summary.BackwardCompatible == summary.TotalOps
	if allCompat {
		lines = append(lines, fmt.Sprintf("  %s All operations backward compatible (%d/%d)", colorize("✓", ansiGreen), summary.BackwardCompatible, summary.TotalOps))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %d/%d operations backward compatible", colorize("⚠", ansiYellow), summary.BackwardCompatible, summary.TotalOps))
	}
	hasBreaking := checkForBreakingChanges(migration.Operations)
	if !hasBreaking {
		lines = append(lines, fmt.Sprintf("  %s No breaking changes", colorize("✓", ansiGreen)))
	} else {
		lines = append(lines, fmt.Sprintf("  %s Contains breaking changes", colorize("✗", ansiRed)))
	}
	rollingSafe := isRollingSafe(summary.MaxRiskScore, allCompat)
	if rollingSafe {
		lines = append(lines, fmt.Sprintf("  %s Rolling deployment safe", colorize("✓", ansiGreen)))
	} else {
		lines = append(lines, fmt.Sprintf("  %s Requires maintenance window", colorize("⚠", ansiYellow)))
	}

	// Build the two-box layout
	mainCard := DrawBox("ANALYSIS RESULTS", lines, width)
	bottomLines := getBottomMessage(summary.MaxRiskScore)
	bottomCard := DrawBox("", bottomLines, width)

	return mainCard + "\n" + bottomCard
}

// wrapText wraps text to fit within the given visible width.
// Each continuation line is indented by the given prefix.
// Leading whitespace on the first line is preserved.
func wrapText(text string, maxWidth int, continuationPrefix string) []string {
	if visibleWidth(text) <= maxWidth {
		return []string{text}
	}

	// Preserve leading whitespace from the original text
	stripped := stripAnsi(text)
	leadingSpaces := ""
	for _, r := range stripped {
		if r == ' ' || r == '\t' {
			leadingSpaces += string(r)
		} else {
			break
		}
	}

	var result []string
	words := strings.Fields(stripped)
	line := leadingSpaces
	for _, word := range words {
		candidate := line
		if candidate != "" && candidate != leadingSpaces {
			candidate += " "
		}
		candidate += word

		if visibleWidth(candidate) > maxWidth && line != "" && line != leadingSpaces {
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

// FormatOperationCard creates a borderless card for a single operation with
// a horizontal divider header and indented content sections.
func FormatOperationCard(op *models.Operation, index int) string {
	width := 76
	riskColor := getRiskColor(op.RiskScore)

	var result strings.Builder

	// ─── Header divider: ─── ADD_COLUMN on users ─────────────────────
	title := fmt.Sprintf("%s on %s", op.Type, op.TableName)
	titleSection := fmt.Sprintf("─── %s ", title)
	titleVisLen := visibleWidth(titleSection)
	remaining := width - titleVisLen
	if remaining < 0 {
		remaining = 0
	}
	result.WriteString(titleSection)
	result.WriteString(strings.Repeat("─", remaining))
	result.WriteString("\n")

	// SQL (indented, word-wrapped, max 3 lines)
	sqlPrefix := "  SQL: "
	sqlText := sqlPrefix + strings.ReplaceAll(op.SQL, "\n", " ")
	innerWidth := width - 2 // 2-char indent for continuation
	sqlLines := wrapText(sqlText, innerWidth, "       ")
	maxSQLLines := 3
	if len(sqlLines) > maxSQLLines {
		sqlLines = sqlLines[:maxSQLLines]
		lastLine := sqlLines[maxSQLLines-1]
		lastVis := visibleWidth(lastLine)
		if lastVis > innerWidth-3 {
			lastLine = truncateWithAnsi(lastLine, innerWidth-3)
		}
		sqlLines[maxSQLLines-1] = lastLine + "..."
	}
	for _, sl := range sqlLines {
		result.WriteString(sl)
		result.WriteString("\n")
	}
	result.WriteString("\n")

	// Risk line: compact single-line risk
	riskLevel := getRiskLevelFromScore(op.RiskScore)
	bar := DrawVisualProgressBar(op.RiskScore, 100, 20, riskColor)
	result.WriteString(fmt.Sprintf("  %s  %s %s %s\n",
		bar,
		colorize(fmt.Sprintf("%d/100", op.RiskScore), riskColor),
		colorize(riskLevel, riskColor),
		getStatusIcon(op.RiskScore)))
	result.WriteString("\n")

	// ── Lock & Timing (merged section) ──
	lockColor := getLockTypeColor(op.LockType)
	lockDesc := getLockDescription(op.LockType)
	result.WriteString(fmt.Sprintf("  Lock:  %s %s\n", colorize(string(op.LockType), lockColor), lockDesc))

	// Queries impact
	queriesStr := estimateQueriesAffected(op.LockType)
	result.WriteString(fmt.Sprintf("         %s\n", queriesStr))

	// Duration line: show lock duration and execution time
	// For most ops they're the same; only show both when different
	lockDurStr := formatLockDuration(op.LockDurationMS)
	execTimeStr := formatExecutionTime(op.EstimatedTimeSeconds)
	lockDurPlain := stripAnsi(lockDurStr)
	execTimePlain := stripAnsi(execTimeStr)

	if lockDurPlain == execTimePlain {
		// Same value — just show once as "Time"
		result.WriteString(fmt.Sprintf("  Time:  %s", execTimeStr))
	} else {
		// Different — show both (e.g. CONCURRENTLY: lock=50ms, build=1.3min)
		result.WriteString(fmt.Sprintf("  Time:  %s (lock held: %s)", execTimeStr, lockDurStr))
	}

	// Table size info
	tableSizeInfo := formatTableSizeInfo(op)
	if tableSizeInfo != "" {
		result.WriteString(fmt.Sprintf(" · %s", tableSizeInfo))
	}
	result.WriteString("\n")
	result.WriteString("\n")

	// ── Compatibility ──
	if op.BackwardCompatible {
		result.WriteString(fmt.Sprintf("  %s Backward compatible\n", colorize("✓", ansiGreen)))
	} else {
		result.WriteString(fmt.Sprintf("  %s May break backward compatibility\n", colorize("⚠", ansiYellow)))
	}
	if !op.RequiresRewrite {
		result.WriteString(fmt.Sprintf("  %s No table rewrite\n", colorize("✓", ansiGreen)))
	} else {
		result.WriteString(fmt.Sprintf("  %s Requires full table rewrite\n", colorize("✗", ansiRed)))
	}

	// ── Recommendations ──
	if len(op.Recommendations) > 0 {
		result.WriteString("\n")
		result.WriteString(fmt.Sprintf("  %s\n", bold("Recommendations")))
		for _, rec := range op.Recommendations {
			wrapped := wrapText("    • "+rec, innerWidth, "      ")
			for _, line := range wrapped {
				result.WriteString(line)
				result.WriteString("\n")
			}
		}
	}

	return result.String()
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
		bar = colorize(bar, color)
	}
	return fmt.Sprintf("  %s %s %d", label, bar, count)
}

// getBottomMessage returns appropriate message lines based on risk score
func getBottomMessage(maxRiskScore int) []string {
	switch {
	case maxRiskScore >= 76:
		return []string{fmt.Sprintf("  %s", colorize(emoji("⚠", "!")+"  CRITICAL: Review carefully before proceeding", ansiRed))}
	case maxRiskScore >= 51:
		return []string{fmt.Sprintf("  %s", colorize(emoji("⚠", "!")+"  Requires thorough testing in staging environment", ansiYellow))}
	case maxRiskScore >= 26:
		return []string{fmt.Sprintf("  %s", colorize(emoji("ℹ", "i")+"  Standard precautions recommended", ansiBlue))}
	default:
		return []string{
			fmt.Sprintf("  %s", colorize(emoji("✨", "*")+" SAFE TO DEPLOY", ansiGreen)),
			fmt.Sprintf("  %s", colorize("This migration can run without downtime.", ansiGreen)),
		}
	}
}

// getStatusIcon returns an emoji icon based on risk score
func getStatusIcon(riskScore int) string {
	switch {
	case riskScore >= 76:
		return emoji("🔴", "[!]")
	case riskScore >= 51:
		return emoji("🟠", "[!]")
	case riskScore >= 26:
		return emoji("🟡", "[-]")
	default:
		return emoji("🟢", "[+]")
	}
}

// getRiskColor returns ANSI color code for risk score, or empty string if colors disabled
func getRiskColor(riskScore int) string {
	if !colorsEnabled() {
		return ""
	}
	switch {
	case riskScore >= 76:
		return ansiRed
	case riskScore >= 51:
		return ansiYellow
	case riskScore >= 26:
		return ansiBlue
	default:
		return ansiGreen
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

// getLockTypeColor returns ANSI color code for lock type, or empty string if colors disabled
func getLockTypeColor(lockType models.LockType) string {
	if !colorsEnabled() {
		return ""
	}
	switch lockType {
	case models.LockTypeAccessExclusive:
		return ansiRed
	case models.LockTypeExclusive:
		return ansiYellow
	case models.LockTypeShare, models.LockTypeShareUpdateExclusive:
		return ansiBlue
	default:
		return ansiGreen
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
		return colorize("< 10ms", ansiGreen)
	} else if durationMS < 100 {
		return colorize(fmt.Sprintf("%dms", durationMS), ansiGreen)
	} else if durationMS < 1000 {
		return colorize(fmt.Sprintf("%dms", durationMS), ansiYellow)
	} else {
		seconds := float64(durationMS) / 1000.0
		return colorize(fmt.Sprintf("%.1fs", seconds), ansiRed)
	}
}

// estimateQueriesAffected estimates number of queries affected by lock
func estimateQueriesAffected(lockType models.LockType) string {
	switch lockType {
	case models.LockTypeAccessExclusive:
		return colorize("All queries blocked", ansiRed)
	case models.LockTypeExclusive:
		return colorize("DDL queries blocked", ansiYellow)
	case models.LockTypeShare:
		return colorize("Writes blocked", ansiBlue)
	case models.LockTypeShareUpdateExclusive:
		return colorize("Writes delayed", ansiBlue)
	default:
		return colorize("0 affected", ansiGreen)
	}
}

// formatExecutionTime formats execution time in a human-readable way
func formatExecutionTime(seconds float64) string {
	if seconds < 1 {
		ms := int(seconds * 1000)
		if ms < 10 {
			return colorize("< 10ms", ansiGreen)
		}
		return colorize(fmt.Sprintf("%dms", ms), ansiGreen)
	} else if seconds < 60 {
		return colorize(fmt.Sprintf("%.1f seconds", seconds), ansiGreen)
	} else if seconds < 300 { // < 5 minutes
		minutes := seconds / 60.0
		return colorize(fmt.Sprintf("%.1f minutes", minutes), ansiYellow)
	} else {
		minutes := seconds / 60.0
		return colorize(fmt.Sprintf("%.1f minutes", minutes), ansiRed)
	}
}

// formatTableSizeInfo formats table size information
func formatTableSizeInfo(op *models.Operation) string {
	if op.RowCount > 0 && op.TableSizeBytes > 0 {
		rows := formatRowCount(op.RowCount)
		size := formatByteSize(op.TableSizeBytes)
		if op.RequiresRewrite {
			return colorize(fmt.Sprintf("%s rows (%s) · rewrite needed", rows, size), ansiRed)
		}
		return fmt.Sprintf("%s rows (%s)", rows, size)
	}
	if op.RequiresRewrite {
		return colorize("rewrite needed", ansiRed)
	}
	return ""
}

// formatRowCount formats a row count in a human-readable way (e.g. 1.0M, 500K)
func formatRowCount(count int64) string {
	switch {
	case count >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(count)/1_000_000_000)
	case count >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	case count >= 1_000:
		return fmt.Sprintf("%.1fK", float64(count)/1_000)
	default:
		return fmt.Sprintf("%d", count)
	}
}

// formatByteSize formats bytes in a human-readable way (e.g. 10.5 GB, 256 MB)
func formatByteSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
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
