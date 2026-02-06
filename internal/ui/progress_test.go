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
