package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNewProgressBar(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := NewProgressBar(buf, 10, "Testing", true)

	if pb.total != 10 {
		t.Errorf("Expected total=10, got %d", pb.total)
	}
	if pb.description != "Testing" {
		t.Errorf("Expected description='Testing', got '%s'", pb.description)
	}
	if !pb.enabled {
		t.Errorf("Expected enabled=true, got false")
	}
}

func TestProgressBarDisabled(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := NewProgressBar(buf, 10, "Testing", false)

	// These should not write anything when disabled
	pb.Increment()
	pb.Increment()
	pb.Finish()

	if buf.Len() != 0 {
		t.Errorf("Expected no output when disabled, got: %s", buf.String())
	}
}

func TestProgressBarIncrement(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := NewProgressBar(buf, 5, "Testing", true)

	// Increment and check output
	pb.Increment()
	output := buf.String()

	if !strings.Contains(output, "Testing") {
		t.Errorf("Expected output to contain 'Testing', got: %s", output)
	}
	if !strings.Contains(output, "[1/5]") {
		t.Errorf("Expected output to contain '[1/5]', got: %s", output)
	}
	if !strings.Contains(output, "20%") {
		t.Errorf("Expected output to contain '20%%', got: %s", output)
	}
}

func TestProgressBarFinish(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := NewProgressBar(buf, 3, "Testing", true)

	// Increment a few times
	pb.Increment()
	pb.Increment()
	pb.Increment()

	// Clear buffer to test Finish
	buf.Reset()
	pb.Finish()

	output := buf.String()
	if !strings.Contains(output, "Complete!") {
		t.Errorf("Expected output to contain 'Complete!', got: %s", output)
	}
	if !strings.Contains(output, "100%") {
		t.Errorf("Expected output to contain '100%%', got: %s", output)
	}
	if !strings.Contains(output, "[3/3]") {
		t.Errorf("Expected output to contain '[3/3]', got: %s", output)
	}
}

func TestProgressBarTiming(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := NewProgressBar(buf, 2, "Testing", true)

	// Wait a bit and increment
	time.Sleep(10 * time.Millisecond)
	pb.Increment()

	output := buf.String()
	// Should show some elapsed time
	if !strings.Contains(output, "s)") {
		t.Errorf("Expected output to show elapsed time, got: %s", output)
	}
}

func TestDrawVisualProgressBar(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		width     int
		fillColor string
		wantFull  int // number of filled blocks
		wantEmpty int // number of empty blocks
	}{
		{
			name:      "50% progress",
			completed: 5,
			total:     10,
			width:     10,
			fillColor: "",
			wantFull:  5,
			wantEmpty: 5,
		},
		{
			name:      "100% progress",
			completed: 10,
			total:     10,
			width:     10,
			fillColor: "",
			wantFull:  10,
			wantEmpty: 0,
		},
		{
			name:      "0% progress",
			completed: 0,
			total:     10,
			width:     10,
			fillColor: "",
			wantFull:  0,
			wantEmpty: 10,
		},
		{
			name:      "with color",
			completed: 3,
			total:     10,
			width:     10,
			fillColor: "\x1b[32m",
			wantFull:  3,
			wantEmpty: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DrawVisualProgressBar(tt.completed, tt.total, tt.width, tt.fillColor)

			// Strip ANSI codes for counting
			stripped := stripAnsiForTest(result)

			// Count filled and empty blocks
			fullCount := strings.Count(stripped, "█")
			emptyCount := strings.Count(stripped, "░")

			if fullCount != tt.wantFull {
				t.Errorf("DrawVisualProgressBar() filled blocks = %d, want %d", fullCount, tt.wantFull)
			}
			if emptyCount != tt.wantEmpty {
				t.Errorf("DrawVisualProgressBar() empty blocks = %d, want %d", emptyCount, tt.wantEmpty)
			}

			// Check total width (count runes, not bytes)
			runeCount := len([]rune(stripped))
			if runeCount != tt.width {
				t.Errorf("DrawVisualProgressBar() width = %d, want %d", runeCount, tt.width)
			}
		})
	}
}

// stripAnsiForTest removes ANSI codes for testing
func stripAnsiForTest(s string) string {
	// Simple ANSI stripping for tests
	result := s
	result = strings.ReplaceAll(result, "\x1b[0m", "")
	result = strings.ReplaceAll(result, "\x1b[32m", "")
	result = strings.ReplaceAll(result, "\x1b[33m", "")
	result = strings.ReplaceAll(result, "\x1b[31m", "")
	return result
}
