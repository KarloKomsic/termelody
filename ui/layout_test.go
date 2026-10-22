package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFitRight(t *testing.T) {
	if got := fitRight("abcdef", 4); got != "abc…" {
		t.Errorf("fitRight = %q, want %q", got, "abc…")
	}
	if got := fitRight("abc", 4); got != "abc" {
		t.Errorf("a fitting string changed: %q", got)
	}
	// Width zero means the terminal size is not known yet, so nothing is cut.
	if got := fitRight("abcdef", 0); got != "abcdef" {
		t.Errorf("unknown width truncated: %q", got)
	}

	// Wide graphemes must not be split in half.
	got := fitRight("音楽ファイル", 5)
	if w := ansi.StringWidth(got); w > 5 {
		t.Errorf("fitRight wide = %q, width %d, want at most 5", got, w)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("fitRight wide = %q, want an ellipsis", got)
	}
}

func TestFitLeft(t *testing.T) {
	const path = "/home/listener/Music/Strana glazba"

	got := fitLeft(path, 14)
	if w := ansi.StringWidth(got); w > 14 {
		t.Errorf("fitLeft = %q, width %d, want at most 14", got, w)
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("fitLeft = %q, want a leading ellipsis", got)
	}
	if !strings.HasSuffix(got, "Strana glazba") {
		t.Errorf("fitLeft = %q, want the tail of the path kept", got)
	}

	if got := fitLeft(path, len(path)+10); got != path {
		t.Errorf("a fitting path changed: %q", got)
	}
}

func TestFitHelpPicksTheFirstFittingLayout(t *testing.T) {
	full := "full help line\nsecond line"
	compact := "compact\nhelp"
	narrow := "short"

	cases := []struct {
		width int
		want  string
	}{
		{0, full},
		{40, full},
		{10, compact},
		{5, narrow},
	}

	for _, c := range cases {
		if got := fitHelp(c.width, full, compact, narrow); got != c.want {
			t.Errorf("width %d: got %q, want %q", c.width, got, c.want)
		}
	}
}

func TestFitHelpClipsWhenNothingFits(t *testing.T) {
	got := fitHelp(6, "full help line\nsecond line", "compact\nhelp")

	for _, line := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(line); w > 6 {
			t.Errorf("line %q is %d wide, want at most 6", line, w)
		}
	}
}
