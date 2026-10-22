package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// fitRight shortens s to width terminal cells, keeping its beginning and
// marking the cut with an ellipsis. Width is measured in cells rather than
// bytes so wide characters survive, and styling is applied by the caller
// afterwards so escape codes cannot be split.
func fitRight(s string, width int) string {
	if width <= 0 || ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

// fitLeft is fitRight for the other end: it keeps the tail of s, which is the
// informative part of a path, and marks the removed beginning with an ellipsis.
func fitLeft(s string, width int) string {
	if width <= 0 {
		return s
	}

	if w := ansi.StringWidth(s); w > width {
		return ansi.TruncateLeft(s, w-width+1, "…")
	}
	return s
}

// widestLine reports the widest line of a block. That is what has to fit the
// terminal, not the block's total width, since lines are independent.
func widestLine(s string) int {
	widest := 0
	for _, line := range strings.Split(s, "\n") {
		widest = max(widest, ansi.StringWidth(line))
	}
	return widest
}

// fitHelp picks the first help layout that fits width. When even the narrowest
// does not, it clips each line: Bubble Tea would clip at the same place anyway,
// but doing it here keeps the block's height honest for the row budget.
func fitHelp(width int, variants ...string) string {
	if width <= 0 {
		return variants[0]
	}

	for _, variant := range variants {
		if widestLine(variant) <= width {
			return variant
		}
	}

	lines := strings.Split(variants[len(variants)-1], "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	return strings.Join(lines, "\n")
}
