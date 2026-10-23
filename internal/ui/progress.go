package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ProgressBar displays a satisfying blue progress bar with live step updates.
type ProgressBar struct {
	mu        sync.Mutex
	w         io.Writer
	total     int
	width     int
	startTime time.Time
	isTTY     bool
	lastStep  int
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(w io.Writer, total int) *ProgressBar {
	return &ProgressBar{
		w:         w,
		total:     total,
		width:     28,
		startTime: time.Now(),
		isTTY:     IsTerminal(w),
	}
}

// SetStep updates the progress bar to current step with a status label.
func (p *ProgressBar) SetStep(step int, status string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if step > p.total {
		step = p.total
	}
	p.lastStep = step
	elapsed := time.Since(p.startTime)
	durStr := FormatDuration(elapsed)

	if !p.isTTY {
		fmt.Fprintf(p.w, "[%d/%d] %s (%s)\n", step, p.total, status, durStr)
		return
	}

	pct := float64(step) / float64(p.total)
	filled := int(pct * float64(p.width))
	if filled > p.width {
		filled = p.width
	}
	empty := p.width - filled

	bar := ElectricBlue + "[" +
		NeonCyan + strings.Repeat("█", filled) +
		MutedGray + strings.Repeat("░", empty) +
		ElectricBlue + "]" + Reset

	pctStr := fmt.Sprintf("%3d%%", int(pct*100))

	fmt.Fprintf(p.w, "\r\033[K%s %s%s%s %s(%s)%s  %s%s%s",
		bar,
		Bold+White, pctStr, Reset,
		MutedGray, durStr, Reset,
		LightGray, status, Reset,
	)
}

// Complete finishes the progress bar with a final success line.
func (p *ProgressBar) Complete(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Since(p.startTime)
	durStr := FormatDuration(elapsed)

	if !p.isTTY {
		fmt.Fprintf(p.w, "✓ %s in %s\n", message, durStr)
		return
	}

	bar := SuccessGreen + "[" + strings.Repeat("█", p.width) + "]" + Reset
	fmt.Fprintf(p.w, "\r\033[K%s %s100%%%s %s(%s)%s  %s%s✓ %s%s\n",
		bar,
		Bold+SuccessGreen, Reset,
		MutedGray, durStr, Reset,
		Bold, SuccessGreen, message, Reset,
	)
}

// FormatDuration formats duration in ms or s.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// CardDetails holds the metadata displayed in the summary card.
type CardDetails struct {
	Title    string
	Class    string
	Output   string
	Size     string
	Target   string
	Duration string
	Memory   string
}

// PrintCard prints a modern summary card.
func PrintCard(w io.Writer, c CardDetails) {
	tty := IsTerminal(w)

	border := func(s string) string {
		if tty {
			return ElectricBlue + s + Reset
		}
		return s
	}
	header := func(s string) string {
		if tty {
			return Bold + NeonCyan + s + Reset
		}
		return s
	}
	label := func(s string) string {
		if tty {
			return Aqua + s + Reset
		}
		return s
	}
	val := func(s string) string {
		if tty {
			return White + s + Reset
		}
		return s
	}
	dim := func(s string) string {
		if tty {
			return MutedGray + s + Reset
		}
		return s
	}

	title := c.Title
	if title == "" {
		title = "Freshly Brewed Native Binary"
	}
	memory := c.Memory
	if memory == "" {
		memory = "~15MB RSS target (zero JVM overhead, sub-5ms boot)"
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, border("╭────────────────────────────────────────────────────────────────────────╮"))
	fmt.Fprintf(w, "%s  ☕ %s%s\n", border("│"), header(title), border(strings.Repeat(" ", 68-len(title)-5)+"│"))
	fmt.Fprintln(w, border("│                                                                        │"))
	if c.Class != "" {
		line := fmt.Sprintf("  • %s  %s", label("Class:   "), val(c.Class))
		fmt.Fprintf(w, "%s%-82s%s\n", border("│"), line, border("│"))
	}
	if c.Target != "" {
		line := fmt.Sprintf("  • %s  %s", label("Target:  "), val(c.Target))
		fmt.Fprintf(w, "%s%-82s%s\n", border("│"), line, border("│"))
	}
	if c.Output != "" {
		line := fmt.Sprintf("  • %s  %s", label("Output:  "), val(c.Output))
		fmt.Fprintf(w, "%s%-82s%s\n", border("│"), line, border("│"))
	}
	if c.Size != "" {
		line := fmt.Sprintf("  • %s  %s", label("Size:    "), val(c.Size))
		fmt.Fprintf(w, "%s%-82s%s\n", border("│"), line, border("│"))
	}
	if c.Duration != "" {
		line := fmt.Sprintf("  • %s  %s", label("Time:    "), val(c.Duration))
		fmt.Fprintf(w, "%s%-82s%s\n", border("│"), line, border("│"))
	}
	lineMem := fmt.Sprintf("  • %s  %s", label("Memory:  "), dim(memory))
	fmt.Fprintf(w, "%s%-82s%s\n", border("│"), lineMem, border("│"))
	fmt.Fprintln(w, border("╰────────────────────────────────────────────────────────────────────────╯"))
	fmt.Fprintln(w)
}
