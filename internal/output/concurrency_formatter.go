package output

import (
	"fmt"
	"io"

	"github.com/iamsr/tapa/pkg/models"
)

// FormatConcurrencyAnalysis formats concurrency impact analysis results
func FormatConcurrencyAnalysis(w io.Writer, analysis *models.ConcurrencyAnalysis) error {
	fmt.Fprintln(w, "\nConcurrency Impact:")
	fmt.Fprintln(w, "─────────────────────────────────────")

	// Impact level and score with color
	impactLevel := analysis.ImpactLevel()
	impactColor := getImpactLevelColor(impactLevel)
	fmt.Fprintf(w, "Impact Level: %s%s%s (Score: %d/100)\n", impactColor, impactLevel, colorReset, analysis.ImpactScore)

	// Concurrency safe indicator
	safeIndicator := "✓ YES"
	safeColor := colorGreen
	if !analysis.ConcurrencySafe {
		safeIndicator = "✗ NO"
		safeColor = colorRed
	}
	fmt.Fprintf(w, "Concurrency Safe: %s%s%s\n", safeColor, safeIndicator, colorReset)

	// Lock Impact
	if analysis.LockImpact != nil {
		fmt.Fprintf(w, "\nLock Details:\n")
		lockColor := LockTypeColor(analysis.LockImpact.LockType)
		fmt.Fprintf(w, "  Type: %s%s%s\n", lockColor, analysis.LockImpact.LockType, colorReset)
		fmt.Fprintf(w, "  Duration: %.1f seconds\n", float64(analysis.LockImpact.EstimatedDurationMS)/1000)

		// What gets blocked
		var blockedOps []string
		if analysis.LockImpact.BlocksReads {
			blockedOps = append(blockedOps, "reads")
		}
		if analysis.LockImpact.BlocksWrites {
			blockedOps = append(blockedOps, "writes")
		}
		if len(blockedOps) > 0 {
			fmt.Fprintf(w, "  Blocks: %s\n", joinWithComma(blockedOps))
		}

		// Blocked query count with red highlighting
		if analysis.LockImpact.EstimatedBlockedCount > 0 {
			fmt.Fprintf(w, "  Estimated blocked queries: %s%d queries%s\n",
				colorRed, analysis.LockImpact.EstimatedBlockedCount, colorReset)
		}

		// Lock acquisition risk
		if analysis.LockImpact.LockAcquisitionRisk != "" {
			riskColor := colorGreen
			switch analysis.LockImpact.LockAcquisitionRisk {
			case "high":
				riskColor = colorRed
			case "medium":
				riskColor = colorYellow
			}
			fmt.Fprintf(w, "  Lock acquisition risk: %s%s%s\n",
				riskColor, analysis.LockImpact.LockAcquisitionRisk, colorReset)
		}

		// Wait time range
		if analysis.LockImpact.WaitTimeRange != "" {
			fmt.Fprintf(w, "  Wait time: %s\n", analysis.LockImpact.WaitTimeRange)
		}
	}

	// Workload Analysis
	if analysis.WorkloadAnalysis != nil {
		fmt.Fprintf(w, "\nWorkload Analysis:\n")
		fmt.Fprintf(w, "  Active connections: %d\n", analysis.WorkloadAnalysis.ActiveConnections)
		fmt.Fprintf(w, "  Queries per second: %.1f\n", analysis.WorkloadAnalysis.QueriesPerSecond)
		fmt.Fprintf(w, "  Table access frequency: %s\n", analysis.WorkloadAnalysis.TableAccessFrequency)

		if analysis.WorkloadAnalysis.PeakLoadPeriod {
			fmt.Fprintf(w, "  %sPeak load period detected!%s\n", colorRed, colorReset)
		}
	}

	// Estimated Downtime
	if analysis.EstimatedDowntimeMS > 0 {
		fmt.Fprintf(w, "\nEstimated Downtime:\n")
		downtimeSeconds := float64(analysis.EstimatedDowntimeMS) / 1000
		downtimeColor := colorGreen
		if downtimeSeconds > 60 {
			downtimeColor = colorRed
		} else if downtimeSeconds > 10 {
			downtimeColor = colorYellow
		}
		fmt.Fprintf(w, "  %s%.1f seconds%s\n", downtimeColor, downtimeSeconds, colorReset)
	}

	// Safer Alternatives
	if len(analysis.SaferAlternatives) > 0 {
		fmt.Fprintf(w, "\nSafer Alternatives:\n")
		for i, alt := range analysis.SaferAlternatives {
			fmt.Fprintf(w, "  %d. %s\n", i+1, alt.Description)
			altLockColor := LockTypeColor(alt.LockType)
			fmt.Fprintf(w, "     Lock: %s%s%s\n", altLockColor, alt.LockType, colorReset)
			fmt.Fprintf(w, "     Impact reduction: %d%%\n", alt.ImpactReduction)

			if len(alt.Steps) > 0 {
				fmt.Fprintf(w, "     Steps:\n")
				for j, step := range alt.Steps {
					fmt.Fprintf(w, "       %d. %s\n", j+1, step)
				}
			}

			if len(alt.Tradeoffs) > 0 {
				fmt.Fprintf(w, "     Tradeoffs:\n")
				for _, tradeoff := range alt.Tradeoffs {
					fmt.Fprintf(w, "       - %s\n", tradeoff)
				}
			}
		}
	}

	// Recommendations
	if len(analysis.Recommendations) > 0 {
		fmt.Fprintf(w, "\nRecommendations:\n")
		for _, rec := range analysis.Recommendations {
			fmt.Fprintf(w, "  • %s\n", rec)
		}
	}

	return nil
}

// getImpactLevelColor returns the appropriate color for an impact level
func getImpactLevelColor(level models.ConcurrencyImpactLevel) string {
	switch level {
	case models.ConcurrencyImpactMinimal, models.ConcurrencyImpactLow:
		return colorGreen
	case models.ConcurrencyImpactMedium:
		return colorYellow
	case models.ConcurrencyImpactHigh, models.ConcurrencyImpactCritical:
		return colorRed
	default:
		return colorReset
	}
}

// joinWithComma joins strings with ", "
func joinWithComma(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
}
