package ui_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JohnEsleyer/barista/internal/ui"
)

func TestProgressBarNonTTY(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := ui.NewProgressBar(buf, 4)

	pb.SetStep(1, "Grinding bytecode...")
	pb.SetStep(2, "Brewing SSA IR...")
	pb.Complete("Complete!")

	out := buf.String()
	if !strings.Contains(out, "[1/4] Grinding bytecode...") {
		t.Errorf("expected step 1 in output: %q", out)
	}
	if !strings.Contains(out, "[2/4] Brewing SSA IR...") {
		t.Errorf("expected step 2 in output: %q", out)
	}
	if !strings.Contains(out, "✓ Complete!") {
		t.Errorf("expected completion in output: %q", out)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := ui.FormatBytes(500); got != "500 B" {
		t.Errorf("FormatBytes(500) = %q, want 500 B", got)
	}
	if got := ui.FormatBytes(2048); got != "2.0 KB" {
		t.Errorf("FormatBytes(2048) = %q, want 2.0 KB", got)
	}
	if got := ui.FormatBytes(3 * 1024 * 1024); got != "3.0 MB" {
		t.Errorf("FormatBytes(3MB) = %q, want 3.0 MB", got)
	}
}

func TestPrintCard(t *testing.T) {
	buf := new(bytes.Buffer)
	ui.PrintCard(buf, ui.CardDetails{
		Title:    "Test Card",
		Class:    "Calculator",
		Output:   "./calculator",
		Size:     "1.5 MB",
		Target:   "linux/amd64",
		Duration: "25ms",
	})
	out := buf.String()
	if !strings.Contains(out, "Calculator") || !strings.Contains(out, "1.5 MB") {
		t.Errorf("card output missing expected details: %q", out)
	}
}

func TestFormatDuration(t *testing.T) {
	if got := ui.FormatDuration(42 * time.Millisecond); got != "42ms" {
		t.Errorf("FormatDuration(42ms) = %q, want 42ms", got)
	}
	if got := ui.FormatDuration(1500 * time.Millisecond); got != "1.50s" {
		t.Errorf("FormatDuration(1500ms) = %q, want 1.50s", got)
	}
}
