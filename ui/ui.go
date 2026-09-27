package ui

import (
	"fmt"

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

type Model struct {
	player  *player.Player
	tracks  []playlist.Track
	cursor  int
	quitting bool
}

func New(p *player.Player, tracks []playlist.Track) Model {
	return Model{
		player: p,
		tracks: tracks,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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

		case "n":
			m.player.Next()
			m.cursor = m.player.Index()

		case "b":
			m.player.Prev()
			m.cursor = m.player.Index()

		case "s":
			m.player.Stop()
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	s := titleStyle.Render("Termelody") + "\n\n"

	for i, t := range m.tracks {
		label := fmt.Sprintf("%s - %s", t.Artist, t.Title)
		if t.Artist == "" && t.Title == "" {
			label = t.Path
		}

		if i == m.cursor {
			s += selectedStyle.Render("> "+label) + "\n"
		} else {
			s += trackStyle.Render("  "+label) + "\n"
		}
	}

	state := m.player.State()
	status := fmt.Sprintf("  [%s]", state)
	s += statusStyle.Render(status) + "\n"

	s += helpStyle.Render("  j/k navigate  enter play  space pause  n/b next/prev  s stop  q quit")

	return s
}
