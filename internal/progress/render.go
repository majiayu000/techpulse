package progress

import (
	"fmt"
	"strings"
)

// render draws the progress bar. Must be called with mutex held.
func (b *Bar) render() {
	percent := 0.0
	filled := 0
	if b.total > 0 {
		percent = float64(b.current) / float64(b.total) * 100
		filled = int(float64(b.width) * float64(b.current) / float64(b.total))
		if filled > b.width {
			filled = b.width
		}
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", b.width-filled)
	fmt.Fprintf(b.out, "\r[%s] %3.0f%% (%d/%d)", bar, percent, b.current, b.total)
}

// renderTaskLine prints a task status line.
func (b *Bar) renderTaskLine(name, status, info string) {
	var icon string
	switch status {
	case "running":
		icon = "⏳"
	case "done":
		icon = "✓"
	case "error":
		icon = "✗"
	default:
		icon = "○"
	}

	// Clear line and print task status
	fmt.Fprint(b.out, "\r\033[K") // Clear current line

	if info != "" {
		fmt.Fprintf(b.out, "  %s %s: %s\n", icon, name, info)
	} else {
		fmt.Fprintf(b.out, "  %s %s\n", icon, name)
	}
}
