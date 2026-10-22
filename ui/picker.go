package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Picker chooses the directory to scan. It is a separate, short-lived program
// rather than a mode of Model: no player exists until a directory is chosen,
// and embedding the browser would give Model a state it can never return from.
type Picker struct {
	dir      string
	all      []string
	entries  []string
	cursor   int
	width    int
	height   int
	err      error
	chosen   string
	canceled bool

	// showHidden is the current view filter. all keeps every directory so
	// toggling is a pure re-filter rather than another disk read.
	showHidden bool

	// notice explains why the picker reopened, such as a folder that held no
	// tracks. It clears as soon as the listener navigates somewhere else, so
	// it cannot describe a folder they are no longer looking at.
	notice string
}

// dirMsg carries the result of reading a directory. Reaching the filesystem in
// Update would block the TUI, so the read runs as a tea.Cmd like every other
// side effect.
type dirMsg struct {
	dir string
	// entries is every subdirectory, including hidden ones; the picker
	// applies its own filter so toggling needs no second read.
	entries []string
	// focus names the subdirectory to leave the cursor on once the read
	// succeeds. Going up would otherwise reset the cursor to the first entry
	// and lose the listener's place.
	focus string
	err   error
}

// NewPicker returns a picker starting in dir. The directory is not read until
// Init, so constructing one stays free of I/O.
func NewPicker(dir string, cfg Config) Picker {
	return Picker{dir: dir, showHidden: cfg.ShowHidden}
}

// WithNotice returns a picker that opens with an explanatory line. It is used
// when a previous choice was rejected, so the listener sees the reason in the
// picker instead of losing the UI to a message they cannot act on.
func (m Picker) WithNotice(notice string) Picker {
	m.notice = notice
	return m
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
		if msg.dir != m.dir {
			// The notice described the folder the picker opened on. Moving
			// elsewhere makes it stale, but the initial load of that same
			// folder must keep it.
			m.notice = ""
		}
		m.dir = msg.dir
		m.all = msg.entries
		m.entries = m.visibleDirs()
		m.cursor = focusIndex(m.entries, msg.focus)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.canceled = true
			return m, tea.Quit

		case "s":
			// s acts on the highlight, not on the folder being browsed: the
			// listener points at what they want, so the cursor decides.
			if len(m.entries) == 0 {
				return m, nil
			}
			m.chosen = filepath.Join(m.dir, m.entries[m.cursor])
			return m, tea.Quit

		case " ":
			// The current folder is not in its own listing, so it needs a key
			// of its own; this also makes an empty folder selectable.
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

		case ".":
			// Keep the cursor on the same folder across the toggle, and let
			// focusIndex fall back to the top when that folder just vanished
			// from view.
			selected := ""
			if len(m.entries) > 0 {
				selected = m.entries[m.cursor]
			}
			m.showHidden = !m.showHidden
			m.entries = m.visibleDirs()
			m.cursor = focusIndex(m.entries, selected)

		case "enter", "right", "l":
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

// Help layouts, widest first. fitHelp picks the first that fits, so a narrow
// terminal trades detail for keeping every key visible.
const (
	pickerHelpFull = "  j/k or ↑/↓ move · enter/→/l open · s scan selected · space scan this folder\n" +
		"  h/← back · g/G top/bottom · . toggle dotfolders · q/esc cancel"
	pickerHelpCompact = "  j/k move · enter open · s scan\n" +
		"  space scan here · h back · . dotfolders\n" +
		"  g/G ends · q quit"
	pickerHelpNarrow = "  enter open · s scan\n" +
		"  space scan here · h back\n" +
		"  . dots · q quit"
)

// View implements tea.Model.
func (m Picker) View() string {
	title := "Termelody - choose a music directory"
	path := m.dir
	if m.width > 0 {
		title = fitRight(title, m.width)
		path = fitLeft(path, max(1, m.width-2))
	}

	above := titleStyle.Render(title) + "\n\n"
	above += trackStyle.Render(path) + "\n\n"

	if m.notice != "" {
		above += errorStyle.Render(m.indented(m.notice)) + "\n\n"
	}

	if m.err != nil {
		above += errorStyle.Render(m.indented(m.err.Error())) + "\n\n"
	}

	below := helpStyle.Render(fitHelp(m.width,
		pickerHelpFull, pickerHelpCompact, pickerHelpNarrow,
	))

	top, end, indicator := m.window(above, below)

	s := above

	// An error already explains the empty list, so the placeholder would only
	// add a second, less useful line.
	if len(m.entries) == 0 && m.err == nil {
		empty := "  (no folders here)"
		if m.width > 0 {
			empty = fitRight(empty, m.width)
		}
		s += statusStyle.Render(empty) + "\n"
	}

	for i := top; i < end; i++ {
		name := m.entries[i]
		if m.width > 0 {
			// Four cells go to padding and the marker, one to the slash.
			name = fitRight(name, max(1, m.width-5))
		}

		label := name + "/"
		if i == m.cursor {
			s += selectedStyle.Render("> "+label) + "\n"
		} else {
			s += trackStyle.Render("  "+label) + "\n"
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

// indented renders a diagnostic line, trimming it to the terminal rather than
// letting the renderer cut it at an arbitrary point.
func (m Picker) indented(text string) string {
	if m.width <= 0 {
		return "  " + text
	}
	return "  " + fitRight(text, max(1, m.width-2))
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

// visibleDirs filters the loaded listing down to what the current setting lets
// through. Hidden entries stay in all, so toggling never needs another read
// and cannot race a directory change.
func (m Picker) visibleDirs() []string {
	if m.showHidden {
		return m.all
	}

	out := make([]string, 0, len(m.all))
	for _, name := range m.all {
		if !strings.HasPrefix(name, ".") {
			out = append(out, name)
		}
	}
	return out
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
