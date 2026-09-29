package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codeberg.org/karlokomsic/termelody/player"
	"codeberg.org/karlokomsic/termelody/playlist"
)

// The help bar prints ↑ and ↓ of its own, so an arrow on its own proves
// nothing about the indicator. Only an arrow followed by a count does.
var indicatorPattern = regexp.MustCompile(`[↑↓] \d+`)

func keyMsg(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

func runeMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func newTestModel(t *testing.T, cfg Config) Model {
	t.Helper()

	return newModelWithTracks(t, cfg, []playlist.Track{
		{Path: "/music/SNAP! - The Power.mp3"},
		{Path: "/music/ZZ Top Sharp Dressed Man.mp3", Artist: "ZZ Top", Title: "Sharp Dressed Man"},
	})
}

// newModelWithTracks builds a Model over the given playlist. Nothing is ever
// played in these tests, so the paths only have to be told apart.
func newModelWithTracks(t *testing.T, cfg Config, tracks []playlist.Track) Model {
	t.Helper()

	p, err := player.New()
	if err != nil {
		t.Skipf("mpv unavailable: %v", err)
	}
	t.Cleanup(func() { p.Close() })

	p.SetPlaylist(tracks)

	return New(p, tracks, cfg)
}

// apply feeds a message through Update and returns the updated Model.
func (m Model) apply(msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestConfigDefaults(t *testing.T) {
	if got := (Config{}).withDefaults().SeekStep; got != defaultSeekStep {
		t.Errorf("zero Config SeekStep = %s, want %s", got, defaultSeekStep)
	}

	// An explicit value must survive rather than being treated as unset.
	if got := (Config{SeekStep: 30 * time.Second}).withDefaults().SeekStep; got != 30*time.Second {
		t.Errorf("explicit SeekStep = %s, want 30s", got)
	}
}

func TestArrowsSeekWithoutMovingSelection(t *testing.T) {
	m := newTestModel(t, Config{})

	m = m.apply(keyMsg(tea.KeyEnter))

	start := m.cursor
	m = m.apply(keyMsg(tea.KeyRight))
	m = m.apply(keyMsg(tea.KeyLeft))

	if m.cursor != start {
		t.Errorf("cursor = %d, want %d: arrows must seek, not navigate", m.cursor, start)
	}
}

func TestNextPrevKeysMoveSelection(t *testing.T) {
	m := newTestModel(t, Config{})
	m = m.apply(keyMsg(tea.KeyEnter))

	m = m.apply(runeMsg('n'))
	if m.cursor != 1 {
		t.Errorf("after n: cursor = %d, want 1", m.cursor)
	}

	m = m.apply(runeMsg('p'))
	if m.cursor != 0 {
		t.Errorf("after p: cursor = %d, want 0", m.cursor)
	}

	// > and < are aliases for n and p.
	m = m.apply(runeMsg('>'))
	if m.cursor != 1 {
		t.Errorf("after >: cursor = %d, want 1", m.cursor)
	}

	m = m.apply(runeMsg('<'))
	if m.cursor != 0 {
		t.Errorf("after <: cursor = %d, want 0", m.cursor)
	}
}

func TestStepWrapsAtBothEnds(t *testing.T) {
	m := newTestModel(t, Config{})

	m.step(-1)
	if m.cursor != 1 {
		t.Errorf("step back from start: cursor = %d, want 1", m.cursor)
	}

	m.step(1)
	if m.cursor != 0 {
		t.Errorf("step forward from end: cursor = %d, want 0", m.cursor)
	}
}

func TestViewShowsConfiguredSeekStep(t *testing.T) {
	m := newTestModel(t, Config{SeekStep: 10 * time.Second})
	m.width, m.height = 100, 24

	out := m.View()

	if !strings.Contains(out, "seek 10s") {
		t.Error("expected help bar to advertise the configured seek step")
	}
}

func TestViewIsCenteredAndOmitsPathPrefix(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width, m.height = 100, 24

	out := m.View()

	if strings.Contains(out, "music/") {
		t.Error("view should not leak the music/ path prefix")
	}
	if !strings.Contains(out, "SNAP! - The Power") {
		t.Error("expected filename fallback in view")
	}
	if !strings.Contains(out, "ZZ Top - Sharp Dressed Man") {
		t.Error("expected metadata label in view")
	}

	// Centering means every non-blank line is indented.
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			t.Errorf("line not indented, so not centered: %q", line)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0:00"},
		{-5 * time.Second, "0:00"},
		{59 * time.Second, "0:59"},
		{time.Minute, "1:00"},
		{90 * time.Second, "1:30"},
		{59*time.Minute + 59*time.Second, "59:59"},
		{time.Hour, "1:00:00"},
		{90 * time.Minute, "1:30:00"},
		{time.Second + 400*time.Millisecond, "0:01"},
	}

	for _, c := range cases {
		if got := formatDuration(c.in); got != c.want {
			t.Errorf("formatDuration(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProgressBarFillsInProportion(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width = 80
	m.pos, m.dur = 30*time.Second, 60*time.Second

	bar := m.progressBar()
	if !strings.Contains(bar, "0:30 / 1:00") {
		t.Errorf("bar missing time labels: %q", bar)
	}

	if drawn := strings.Count(bar, "━"); drawn != 40 {
		t.Errorf("bar drew %d segments, want 40", drawn)
	}
}

func TestBarSegments(t *testing.T) {
	cases := []struct {
		width    int
		fraction float64
		wantFill int
	}{
		{40, 0, 0},
		{40, -1, 0},
		{40, 0.5, 20},
		{40, 1, 40},
		{40, 1.5, 40},
		{40, 0.0001, 1}, // must not round away to nothing
		{10, 0.95, 9},
		{1, 0.5, 1},
	}

	for _, c := range cases {
		filled, left := barSegments(c.width, c.fraction)
		if filled != c.wantFill {
			t.Errorf("barSegments(%d, %v) filled = %d, want %d", c.width, c.fraction, filled, c.wantFill)
		}
		if filled+left != c.width {
			t.Errorf("barSegments(%d, %v) total = %d, want %d", c.width, c.fraction, filled+left, c.width)
		}
	}
}

func TestProgressBarHidesAtFullAndZero(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width = 80

	// Wholly played: the filled run must reach the end and leave nothing.
	m.pos, m.dur = 60*time.Second, 60*time.Second
	if s := m.progressBar(); !strings.Contains(s, "1:00 / 1:00") {
		t.Errorf("missing labels: %q", s)
	}

	// A position past the end must not overflow the bar.
	m.pos = 90 * time.Second
	if drawn := strings.Count(m.progressBar(), "━"); drawn != 40 {
		t.Errorf("bar drew %d segments at full progress, want 40", drawn)
	}
}

func TestProgressBarOmitsBarWhenDurationUnknown(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width = 80
	m.pos, m.dur = 42*time.Second, 0

	bar := m.progressBar()
	if strings.Contains(bar, "━") {
		t.Errorf("drew a bar with no duration: %q", bar)
	}
	if !strings.Contains(bar, "0:42") {
		t.Errorf("should still show elapsed time: %q", bar)
	}
}

func TestProgressBarFallsBackToLabelsWhenNarrow(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width = 20
	m.pos, m.dur = 30*time.Second, 60*time.Second

	bar := m.progressBar()
	if strings.Contains(bar, "━") {
		t.Errorf("drew a bar in a %d column terminal: %q", m.width, bar)
	}
	if !strings.Contains(bar, "0:30 / 1:00") {
		t.Errorf("should fall back to labels: %q", bar)
	}
}

func TestTickReadsPositionAndReArms(t *testing.T) {
	m := newTestModel(t, Config{})

	next, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("tick did not schedule another")
	}
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	if updated.pos != m.player.Position() {
		t.Errorf("pos = %s, want %s", updated.pos, m.player.Position())
	}
}

func TestUpDownArrowsMoveTheCursor(t *testing.T) {
	m := newTestModel(t, Config{})

	// The arrows were already wired up; only the help bar left them out, so
	// this guards the claim it now makes.
	m = m.apply(keyMsg(tea.KeyDown))
	if m.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.cursor)
	}

	m = m.apply(keyMsg(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("after up: cursor = %d, want 0", m.cursor)
	}

	// Bound at both ends, exactly like j and k.
	m = m.apply(keyMsg(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("at the top: cursor = %d, want 0", m.cursor)
	}

	for range len(m.tracks) + 2 {
		m = m.apply(keyMsg(tea.KeyDown))
	}
	if m.cursor != len(m.tracks)-1 {
		t.Errorf("at the bottom: cursor = %d, want %d", m.cursor, len(m.tracks)-1)
	}
}

func TestHelpAdvertisesNavigationKeys(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width, m.height = 100, 24

	out := m.View()

	for _, want := range []string{"j/k or ↑/↓ move", "n/p or >/< next/prev"} {
		if !strings.Contains(out, want) {
			t.Errorf("help bar does not advertise %q:\n%s", want, out)
		}
	}
}

// endFile builds the event mpv sends when a file stops playing, whatever the
// reason for it stopping.
func endFile(reason string) eventMsg {
	return eventMsg(player.Event{
		Name: "end-file",
		Raw:  []byte(`{"event":"end-file","reason":"` + reason + `"}`),
	})
}

func TestAutoAdvancePlaysTheNextTrack(t *testing.T) {
	m := newTestModel(t, Config{})

	m = m.apply(endFile("eof"))

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1: the next track was not selected", m.cursor)
	}
	if m.player.Index() != 1 {
		t.Errorf("player index = %d, want 1: the next track was not played", m.player.Index())
	}
}

func TestAutoAdvanceIgnoresAnInterruptedFile(t *testing.T) {
	m := newTestModel(t, Config{})

	// This is the payload pressing n and pressing s both produce, so a
	// change here means the playlist jumps ahead on every manual track
	// change. See ipc.TestInterruptedFileIsNotEOF for where it comes from.
	m = m.apply(endFile("stop"))

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0: an interrupted file advanced the playlist", m.cursor)
	}
	if m.player.Index() != 0 {
		t.Errorf("player index = %d, want 0: an interrupted file advanced the playlist", m.player.Index())
	}
}

func TestAutoAdvanceStopsAfterTheLastTrack(t *testing.T) {
	m := newTestModel(t, Config{})

	// Two tracks, so stepping once lands on the last one.
	m.step(1)

	m = m.apply(endFile("eof"))

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1: the playlist wrapped instead of stopping", m.cursor)
	}
	if m.player.Index() != 1 {
		t.Errorf("player index = %d, want 1: the playlist wrapped instead of stopping", m.player.Index())
	}
}

// manyTracks builds a playlist whose labels can be told apart in the render.
func manyTracks(n int) []playlist.Track {
	tracks := make([]playlist.Track, n)
	for i := range tracks {
		tracks[i] = playlist.Track{Path: fmt.Sprintf("track-%03d.mp3", i)}
	}
	return tracks
}

func TestWindowTop(t *testing.T) {
	cases := []struct {
		name                   string
		cursor, total, visible int
		want                   int
	}{
		{"everything fits", 7, 5, 10, 0},
		{"at the top", 0, 100, 10, 0},
		{"near the top", 4, 100, 10, 0},
		{"in the middle", 50, 100, 10, 45},
		{"near the end", 98, 100, 10, 90},
		{"at the end", 99, 100, 10, 90},
	}

	for _, c := range cases {
		if got := windowTop(c.cursor, c.total, c.visible); got != c.want {
			t.Errorf("%s: windowTop(%d, %d, %d) = %d, want %d",
				c.name, c.cursor, c.total, c.visible, got, c.want)
		}
	}
}

// The whole window exists to keep the cursor on screen, so that is the
// property worth checking exhaustively rather than at a few points.
func TestWindowTopAlwaysShowsTheCursor(t *testing.T) {
	for total := range 60 {
		for visible := 1; visible <= 25; visible++ {
			for cursor := range total {
				top := windowTop(cursor, total, visible)
				shown := min(visible, total)

				if top < 0 {
					t.Fatalf("total=%d visible=%d cursor=%d: top = %d, want >= 0",
						total, visible, cursor, top)
				}
				if top+shown > total {
					t.Fatalf("total=%d visible=%d cursor=%d: window [%d,%d) runs past the end",
						total, visible, cursor, top, top+shown)
				}
				if cursor < top || cursor >= top+shown {
					t.Fatalf("total=%d visible=%d cursor=%d: window is [%d,%d), cursor is hidden",
						total, visible, cursor, top, top+shown)
				}
			}
		}
	}
}

func TestIndicatorText(t *testing.T) {
	cases := []struct {
		top, end, total int
		want            string
	}{
		{0, 10, 100, "↓ 90"},
		{45, 55, 100, "↑ 45 · ↓ 45"},
		{90, 100, 100, "↑ 90"},
	}

	for _, c := range cases {
		if got := indicatorText(c.top, c.end, c.total); got != c.want {
			t.Errorf("indicatorText(%d, %d, %d) = %q, want %q",
				c.top, c.end, c.total, got, c.want)
		}
	}
}

func TestViewWindowsTheTrackList(t *testing.T) {
	m := newModelWithTracks(t, Config{}, manyTracks(200))
	m.width, m.height = 100, 30
	m.cursor = 150

	out := m.View()

	// lipgloss.Place pads a block up to the requested height but never
	// clips one, so a view one line too tall spills past the terminal
	// rather than being cut. This is the assertion that catches it.
	if got := lipgloss.Height(out); got != m.height {
		t.Errorf("rendered %d lines, want exactly %d", got, m.height)
	}

	if strings.Contains(out, "track-000") {
		t.Error("a track far outside the window was rendered, so the list is not being windowed")
	}
	if !strings.Contains(out, "track-150") {
		t.Error("the cursor's own track was not rendered")
	}
	if !indicatorPattern.MatchString(out) {
		t.Error("expected an indicator for the tracks left out")
	}
}

func TestViewShowsAllTracksWhenTheyFit(t *testing.T) {
	m := newModelWithTracks(t, Config{}, manyTracks(20))
	m.width, m.height = 100, 40
	m.cursor = 19

	out := m.View()

	for i := range 20 {
		if !strings.Contains(out, fmt.Sprintf("track-%03d", i)) {
			t.Errorf("track-%03d missing from a window that has room for all 20", i)
		}
	}
	if indicatorPattern.MatchString(out) {
		t.Error("nothing is hidden, so there should be no indicator")
	}
	if got := lipgloss.Height(out); got != m.height {
		t.Errorf("rendered %d lines, want exactly %d", got, m.height)
	}
}
