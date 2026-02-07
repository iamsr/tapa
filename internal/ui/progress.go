package ui

import (
	"fmt"
	"io"
	"time"
)

// ProgressBar displays progress during file parsing operations
type ProgressBar struct {
	writer      io.Writer
	total       int
	current     int
	startTime   time.Time
	description string
	enabled     bool
}

// NewProgressBar creates a new progress bar instance
func NewProgressBar(w io.Writer, total int, description string, enabled bool) *ProgressBar {
	return &ProgressBar{
		writer:      w,
		total:       total,
		current:     0,
		startTime:   time.Now(),
		description: description,
		enabled:     enabled,
	}
}

// Increment increases the progress counter by one
func (p *ProgressBar) Increment() {
	if !p.enabled {
		return
	}
	p.current++
	p.render()
}

// render displays the current progress state
func (p *ProgressBar) render() {
	if !p.enabled {
		return
	}

	percent := 0
	if p.total > 0 {
		percent = (p.current * 100) / p.total
	}

	elapsed := time.Since(p.startTime)
	fmt.Fprintf(p.writer, "\r%s: [%d/%d] %d%% (%.1fs)",
		p.description,
		p.current,
		p.total,
		percent,
		elapsed.Seconds())
}

// Finish completes the progress bar display
func (p *ProgressBar) Finish() {
	if !p.enabled {
		return
	}

	elapsed := time.Since(p.startTime)
	fmt.Fprintf(p.writer, "\r%s: [%d/%d] 100%% (%.1fs) - Complete!\n",
		p.description,
		p.total,
		p.total,
		elapsed.Seconds())
}

// DrawVisualProgressBar creates a visual progress bar string
func DrawVisualProgressBar(completed int, total int, width int, fillColor string) string {
	if total <= 0 {
		return ""
	}

	// Calculate filled portion
	filled := (completed * width) / total
	if filled > width {
		filled = width
	}

	// Build progress bar
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	// Apply color if provided and colors are enabled
	if fillColor != "" && colorsEnabled() {
		bar = fillColor + bar + ansiReset
	}

	return bar
}
