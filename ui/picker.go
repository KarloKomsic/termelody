package ui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Picker chooses the directory to scan. It is a separate, short-lived program
// rather than a mode of Model: no player exists until a directory is chosen,
// and embedding the browser would give Model a state it can never return from.
type Picker struct {
	dir      string
	entries  []string
	cursor   int
	width    int
	height   int
	err      error
	chosen   string
	canceled bool
}

// dirMsg carries the result of reading a directory. Reaching the filesystem in
// Update would block the TUI, so the read runs as a tea.Cmd like every other
// side effect.
type dirMsg struct {
	dir     string
	entries []string
	// focus names the subdirectory to leave the cursor on once the read
	// succeeds. Going up would otherwise reset the cursor to the first entry
	// and lose the listener's place.
	focus string
	err   error
}

// NewPicker returns a picker starting in dir. The directory is not read until
// Init, so constructing one stays free of I/O.
func NewPicker(dir string) Picker {
	return Picker{dir: dir}
}

// Init implements tea.Model.
func (m Picker) Init() tea.Cmd {
	return loadDir(m.dir)
}

// Update implements tea.Model.
func (m Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case dirMsg:
		m.err = msg.err
		if msg.err != nil {
			// Stay where we are: entering a directory that cannot be read
			// would leave the picker showing nothing and no way back.
			return m, nil
		}
		m.dir = msg.dir
		m.entries = msg.entries
		m.cursor = focusIndex(msg.entries, msg.focus)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.canceled = true
			return m, tea.Quit

		case "s":
			m.chosen = m.dir
			return m, tea.Quit

		case "j", "down":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

		case "G":
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}

		case "enter":
			if len(m.entries) == 0 {
				return m, nil
			}
			return m, loadDir(filepath.Join(m.dir, m.entries[m.cursor]))

		case "h", "left", "backspace":
			parent := filepath.Dir(m.dir)
			if parent == m.dir {
				// The filesystem root has no parent, so there is nowhere to go.
				return m, nil
			}
			return m, loadDirFocus(parent, filepath.Base(m.dir))
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m Picker) View() string {
	above := titleStyle.Render("Termelody - choose a music directory") + "\n\n"
	above += trackStyle.Render(m.dir) + "\n\n"

	if m.err != nil {
		above += errorStyle.Render("  "+m.err.Error()) + "\n\n"
	}

	below := helpStyle.Render(
		"  j/k or ↑/↓ move · enter open · h back · s select this folder\n" +
			"  g/G top/bottom · q/esc cancel",
	)

	top, end, indicator := m.window(above, below)

	s := above

	// An error already explains the empty list, so the placeholder would only
	// add a second, less useful line.
	if len(m.entries) == 0 && m.err == nil {
		s += statusStyle.Render("  (no folders here)") + "\n"
	}

	for i := top; i < end; i++ {
		name := m.entries[i] + "/"
		if i == m.cursor {
			s += selectedStyle.Render("> "+name) + "\n"
		} else {
			s += trackStyle.Render("  "+name) + "\n"
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

// Chosen reports the directory the listener selected, or "" when the picker
// was canceled or never finished.
func (m Picker) Chosen() string {
	return m.chosen
}

// Canceled reports whether the listener backed out instead of choosing.
func (m Picker) Canceled() bool {
	return m.canceled
}

// window reports which slice of the directory list fits between the fixed
// chrome. The budget is taken from the assembled blocks for the same reason
// Model.trackWindow does: lipgloss.Place pads but never clips, so a view one
// line too tall spills past the bottom of the terminal.
func (m Picker) window(above, below string) (top, end int, indicator string) {
	top, end = 0, len(m.entries)

	if m.width == 0 || m.height == 0 {
		return top, end, ""
	}

	room := m.height - lipgloss.Height(above+below)
	if len(m.entries) <= room {
		return top, end, ""
	}

	// One of the room's lines goes to the indicator.
	visible := max(1, room-1)
	top = windowTop(m.cursor, len(m.entries), visible)
	end = min(top+visible, len(m.entries))

	return top, end, indicatorText(top, end, len(m.entries))
}

// loadDir reads a directory's subdirectories. The read happens inside the
// returned command so Update never touches the filesystem.
func loadDir(dir string) tea.Cmd {
	return loadDirFocus(dir, "")
}

// loadDirFocus is loadDir with a subdirectory to put the cursor on, which is
// how backing out of a folder keeps that folder selected.
func loadDirFocus(dir, focus string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return dirMsg{dir: dir, focus: focus, err: err}
		}

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			// Symlinks are skipped, matching the scanner: following one that
			// points at an ancestor is how a walk runs forever.
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}

		return dirMsg{dir: dir, entries: names, focus: focus}
	}
}

// focusIndex returns the position of name, or the first entry when it is
// absent, so a vanished directory still leaves a valid cursor.
func focusIndex(entries []string, name string) int {
	for i, entry := range entries {
		if entry == name {
			return i
		}
	}
	return 0
}
