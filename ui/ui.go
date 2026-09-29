package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codeberg.org/karlokomsic/termelody/player"
	"codeberg.org/karlokomsic/termelody/playlist"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	trackStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("205")).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	// The indicator deliberately has no margins. Every other block here
	// separates itself with MarginTop, and that would add a line the track
	// budget has not accounted for, pushing the view past the terminal
	// height -- which lipgloss.Place will happily let happen, since it pads
	// but never clips.
	indicatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(2)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true).
			MarginTop(1)

	barDoneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	barLeftStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// defaultSeekStep is how far the arrow keys jump when no config overrides it.
const defaultSeekStep = 5 * time.Second

// Config holds tunables for the TUI. The zero value is valid and yields
// defaults, so callers can start from an empty Config and fill in only what
// they care about. Every new preference belongs here rather than in main.
type Config struct {
	// SeekStep is how many seconds the arrow keys skip forwards or back.
	SeekStep time.Duration
}

// withDefaults fills in any options left at their zero value.
func (c Config) withDefaults() Config {
	if c.SeekStep <= 0 {
		c.SeekStep = defaultSeekStep
	}
	return c
}

// tickInterval is how often the progress readout refreshes. It is far slower
// than the rate mpv reports position, because a redraw that often buys nothing
// visible and costs a full repaint of the screen.
const tickInterval = 200 * time.Millisecond

// tickMsg asks the model to refresh the progress readout.
type tickMsg time.Time

// tick schedules the next refresh.
func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Model holds the entire state of the TUI.
type Model struct {
	player   *player.Player
	tracks   []playlist.Track
	cfg      Config
	cursor   int
	width    int
	height   int
	pos      time.Duration
	dur      time.Duration
	mpvErr   error
	quitting bool
}

// eventMsg carries a playback event into the Bubble Tea loop. Events have to
// arrive as messages rather than being read during View, so that every change
// to the model happens on Bubble Tea's single goroutine.
type eventMsg player.Event

// mpvGoneMsg reports that the mpv process is no longer available.
type mpvGoneMsg struct{ err error }

// waitForEvent blocks until mpv reports something, or until it exits. Because
// it is a tea.Cmd it runs on Bubble Tea's own goroutine, and returning a fresh
// command from Update re-arms the listener.
func waitForEvent(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev, ok := <-p.Events():
			if !ok {
				return mpvGoneMsg{}
			}
			return eventMsg(ev)
		case <-p.Done():
			return mpvGoneMsg{}
		}
	}
}

// New builds a Model for the given player and track list. A zero Config is
// valid and selects the defaults.
func New(p *player.Player, tracks []playlist.Track, cfg Config) Model {
	return Model{
		player: p,
		tracks: tracks,
		cfg:    cfg.withDefaults(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForEvent(m.player), tick())
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Position is read here rather than carried in as an event, so that
		// the refresh rate is a choice of the UI instead of a side effect of
		// how often mpv happens to report.
		m.pos = m.player.Position()
		m.dur = m.player.Duration()
		return m, tick()

	case eventMsg:
		// The player has already folded this event into its state, so the
		// only thing left to decide is whether whatever just ended should
		// be followed by something else.
		if player.EndedNaturally(player.Event(msg)) {
			m.advance()
		}
		return m, waitForEvent(m.player)

	case mpvGoneMsg:
		// Without mpv there is nothing to control, so stop asking it.
		m.mpvErr = errors.New("mpv exited unexpectedly")
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "j", "down":
			if m.cursor < len(m.tracks)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "enter":
			if err := m.player.PlayIndex(m.cursor); err != nil {
				return m, nil
			}

		case " ":
			// mpv reports the result, so there is no local decision to
			// make and no way for this to disagree with reality.
			m.player.TogglePause()

		case "left":
			m.seek(-m.cfg.SeekStep)

		case "right":
			m.seek(m.cfg.SeekStep)

		case "n", ">":
			m.step(1)

		case "p", "<":
			m.step(-1)

		case "s":
			m.player.Stop()
		}
	}

	return m, nil
}

// progressBar renders the position within the track. When mpv does not report
// a duration, which it does not for streams and some containers, only the
// elapsed time is shown rather than a bar that would be meaningless.
func (m Model) progressBar() string {
	const (
		indent   = "  "
		maxWidth = 40
	)

	elapsed := formatDuration(m.pos)

	if m.dur <= 0 {
		return statusStyle.Render(indent + elapsed)
	}

	// The bar shares the line with the time labels, so what is left of the
	// available width is what the bar itself can use.
	labels := fmt.Sprintf("%s / %s", elapsed, formatDuration(m.dur))
	width := m.width - lipgloss.Width(labels) - lipgloss.Width(indent) - 2
	if m.width == 0 {
		width = maxWidth
	}
	if width < 10 {
		// Too narrow to draw a bar honestly, so the numbers stand alone.
		return statusStyle.Render(indent + labels)
	}
	if width > maxWidth {
		width = maxWidth
	}

	filled, left := barSegments(width, m.fraction())

	bar := barDoneStyle.Render(strings.Repeat("━", filled)) +
		barLeftStyle.Render(strings.Repeat("━", left))

	return statusStyle.Render(indent+labels+"  ") + bar
}

// barSegments splits a bar of the given width into the part already played and
// the part remaining, keeping the total exactly width so the bar never shifts
// as playback advances.
func barSegments(width int, fraction float64) (filled, left int) {
	switch {
	case fraction <= 0:
		return 0, width
	case fraction >= 1:
		return width, 0
	}

	filled = int(float64(width) * fraction)
	// A fraction just above zero must not round away to nothing, or a
	// barely-started track would show an untouched bar.
	if filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return filled, width - filled
}

// fraction reports playback progress, guarding against a position that briefly
// runs past the duration while a new track is settling.
func (m Model) fraction() float64 {
	if m.dur <= 0 {
		return 0
	}
	return min(float64(m.pos)/float64(m.dur), 1)
}

// formatDuration renders a duration as m:ss, or h:mm:ss once it passes an hour.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	total := int(d.Round(time.Second).Seconds())
	seconds := total % 60
	minutes := (total / 60) % 60
	hours := total / 3600

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	above := titleStyle.Render("Termelody") + "\n\n"

	below := statusStyle.Render(fmt.Sprintf("  [%s]", m.player.State())) + "\n"
	below += m.progressBar() + "\n"

	if m.mpvErr != nil {
		below += errorStyle.Render(fmt.Sprintf("  %v", m.mpvErr)) + "\n"
	}

	below += helpStyle.Render(fmt.Sprintf(
		"  j/k or ↑/↓ move · enter play · space pause · ←/→ seek %ds · n/p or >/< next/prev · s stop · q quit",
		int(m.cfg.SeekStep.Seconds()),
	))

	top, end, indicator := m.trackWindow(above, below)

	s := above
	for i := top; i < end; i++ {
		if i == m.cursor {
			s += selectedStyle.Render("> "+trackLabel(m.tracks[i])) + "\n"
		} else {
			s += trackStyle.Render("  "+trackLabel(m.tracks[i])) + "\n"
		}
	}

	if indicator != "" {
		s += indicatorStyle.Render(indicator) + "\n"
	}

	s += below

	if m.width == 0 || m.height == 0 {
		return s
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s)
}

// trackWindow reports which slice of the track list fits between the fixed
// chrome, and what to say about anything left out.
//
// The budget is counted from the assembled chrome rather than from the sum of
// its pieces, because joining two blocks with a newline is one line and not
// two. It has to be exact: lipgloss.Place pads a block up to the requested
// height but never clips one, so a view that is a single line too tall spills
// past the bottom of the terminal instead of being cut.
func (m Model) trackWindow(above, below string) (top, end int, indicator string) {
	top, end = 0, len(m.tracks)

	if m.width == 0 || m.height == 0 {
		// The size has not arrived yet, so there is nothing to fit into.
		return top, end, ""
	}

	room := m.height - lipgloss.Height(above+below)
	if len(m.tracks) <= room {
		return top, end, ""
	}

	// One of the room's lines goes to the indicator, so the list gets one
	// fewer. A room too small for that still shows one track, since an empty
	// playlist view helps nobody.
	visible := max(1, room-1)
	top = windowTop(m.cursor, len(m.tracks), visible)
	end = min(top+visible, len(m.tracks))

	return top, end, indicatorText(top, end, len(m.tracks))
}

// windowTop returns the first index to show so that the cursor sits near the
// middle of a window of visible rows. It holds no state: the offset is
// derived from the cursor every time, so it cannot fall out of sync with the
// several places that move the cursor.
func windowTop(cursor, total, visible int) int {
	if visible >= total {
		return 0
	}

	top := cursor - visible/2
	switch {
	case top < 0:
		return 0
	case top > total-visible:
		return total - visible
	default:
		return top
	}
}

// indicatorText describes how many tracks fall outside the window. At either
// edge one side has nothing to report and is left out, so the line never
// points a direction that is empty.
func indicatorText(top, end, total int) string {
	above, below := top, total-end

	switch {
	case above > 0 && below > 0:
		return fmt.Sprintf("↑ %d · ↓ %d", above, below)
	case above > 0:
		return fmt.Sprintf("↑ %d", above)
	default:
		return fmt.Sprintf("↓ %d", below)
	}
}

// seek shifts playback by delta, bounded by cfg.SeekStep at the call site.
func (m *Model) seek(delta time.Duration) {
	m.player.Seek(delta)
}

// advance plays the track after the one that ended on its own. It stops at
// the end of the playlist instead of wrapping: pressing n is a choice the
// listener made, while looping forever with no repeat toggle is not one they
// can undo.
//
// The cursor is moved to follow playback because it may have been parked
// elsewhere while the track was playing, and a highlight that disagrees with
// what is playing is worse than no highlight at all.
func (m *Model) advance() {
	if len(m.tracks) == 0 || m.player.Index() >= len(m.tracks)-1 {
		return
	}

	if err := m.player.Next(); err != nil {
		m.mpvErr = err
		return
	}
	m.cursor = m.player.Index()
}

// step moves the cursor by delta tracks, wrapping at both ends, and plays
// whatever it lands on. The cursor drives navigation so the selection stays
// in sync with what is playing.
func (m *Model) step(delta int) {
	if len(m.tracks) == 0 {
		return
	}

	next := (m.cursor + delta) % len(m.tracks)
	if next < 0 {
		next += len(m.tracks)
	}

	m.cursor = next
	m.player.PlayIndex(next)
}

// trackLabel prefers metadata, falling back to the filename when a file has
// no embedded tags. Showing the full path would be noisy in a fixed-width list.
func trackLabel(t playlist.Track) string {
	if t.Artist != "" || t.Title != "" {
		return fmt.Sprintf("%s - %s", t.Artist, t.Title)
	}

	name := filepath.Base(t.Path)
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}

	return name
}
