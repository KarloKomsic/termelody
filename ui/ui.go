package ui

import (
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

// Model holds the entire state of the TUI.
type Model struct {
	player   *player.Player
	tracks   []playlist.Track
	cfg      Config
	cursor   int
	width    int
	height   int
	quitting bool
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
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
			switch m.player.State() {
			case player.StatePlaying:
				m.player.Pause()
			case player.StatePaused:
				m.player.Resume()
			}

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

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	s := titleStyle.Render("Termelody") + "\n\n"

	for i, t := range m.tracks {
		if i == m.cursor {
			s += selectedStyle.Render("> "+trackLabel(t)) + "\n"
		} else {
			s += trackStyle.Render("  "+trackLabel(t)) + "\n"
		}
	}

	s += statusStyle.Render(fmt.Sprintf("  [%s]", m.player.State())) + "\n"
	s += helpStyle.Render(fmt.Sprintf(
		"  j/k move · enter play · space pause · ←/→ seek %ds · n/p next/prev · s stop · q quit",
		int(m.cfg.SeekStep.Seconds()),
	))

	if m.width == 0 || m.height == 0 {
		return s
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s)
}

// seek shifts playback by delta, bounded by cfg.SeekStep at the call site.
func (m *Model) seek(delta time.Duration) {
	m.player.Seek(delta)
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
