package ui

import (
	"fmt"
	"io"
	"os"
)

// ANSI Styling & Color constants.
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	// Blue & Cyan Palette for Barista
	ElectricBlue = "\033[38;5;39m"  // Vibrant bright sky blue
	DeepSkyBlue  = "\033[38;5;33m"  // Deep saturated blue
	RoyalBlue    = "\033[38;5;27m"  // Darker royal blue
	NeonCyan     = "\033[38;5;51m"  // High-visibility cyan
	Aqua         = "\033[38;5;45m"  // Soft glowing aqua

	// Status & Accent Colors
	SuccessGreen = "\033[38;5;48m"  // Crisp mint green
	WarningAmber = "\033[38;5;214m" // Warm amber
	ErrorRed     = "\033[38;5;196m" // Clean red
	MutedGray    = "\033[38;5;244m" // Subtle slate gray
	LightGray    = "\033[38;5;250m" // Readable off-white
	White        = "\033[97m"       // Crisp pure white
)

// IsTerminal reports whether w is an interactive terminal that supports ANSI escapes.
func IsTerminal(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

// FormatBytes formats a byte size into a human-readable string.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
