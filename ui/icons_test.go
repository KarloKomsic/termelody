package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestParseIconSet(t *testing.T) {
	for _, want := range []IconSet{IconsUnicode, IconsNerd, IconsNone} {
		got, err := ParseIconSet(string(want))
		if err != nil {
			t.Errorf("ParseIconSet(%q): %v", want, err)
			continue
		}
		if got != want {
			t.Errorf("ParseIconSet(%q) = %q, want %q", want, got, want)
		}
	}

	for _, bad := range []string{"", "fancy", "NERD"} {
		if got, err := ParseIconSet(bad); err == nil {
			t.Errorf("ParseIconSet(%q) = %q, want an error", bad, got)
		}
	}
}

func TestIconSetNextCycles(t *testing.T) {
	cases := []struct {
		from  IconSet
		delta int
		want  IconSet
	}{
		{IconsUnicode, 1, IconsNerd},
		{IconsNerd, 1, IconsNone},
		{IconsNone, 1, IconsUnicode},
		{IconsUnicode, -1, IconsNone},
		{IconsNone, -1, IconsNerd},
		{IconsNerd, 2, IconsUnicode},
		{IconsNerd, 0, IconsNerd},
		// A value the loader would reject still has somewhere to move from.
		{IconSet("bogus"), 1, IconsNerd},
		{IconSet("bogus"), 0, IconsUnicode},
	}

	for _, c := range cases {
		if got := c.from.next(c.delta); got != c.want {
			t.Errorf("%q.next(%d) = %q, want %q", c.from, c.delta, got, c.want)
		}
	}
}

func TestIconPrefixes(t *testing.T) {
	cases := []struct {
		set           IconSet
		folder, track string
	}{
		{IconsUnicode, "\U0001F5C0 ", "♪ "},
		{IconsNerd, "\uf07b ", "\uf001 "},
		{IconsNone, "", ""},
		// An unrecognized value falls back to the portable set rather than
		// dropping the icon column, so a stale config cannot blank rows.
		{IconSet("bogus"), "\U0001F5C0 ", "♪ "},
	}

	for _, c := range cases {
		folder, track := c.set.folderPrefix(), c.set.trackPrefix()
		if folder != c.folder {
			t.Errorf("%q folder prefix = %q, want %q", c.set, folder, c.folder)
		}
		if track != c.track {
			t.Errorf("%q track prefix = %q, want %q", c.set, track, c.track)
		}

		// The render budget assumes each prefix is either absent or one glyph
		// plus its separating space.
		for _, prefix := range []string{folder, track} {
			if prefix != "" && lipgloss.Width(prefix) != 2 {
				t.Errorf("%q prefix %q is %d cells, want 2", c.set, prefix, lipgloss.Width(prefix))
			}
		}
	}
}
