package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// setting identifies one row of the options screen. The declaration order is
// the order on screen, so cursor movement needs no separate table.
type setting int

const (
	settingSeek setting = iota
	settingIcons
	settingShowHidden
	settingVolumeStep
	settingCount
)

// settingLabelWidth aligns every value in one column. It is derived from the
// longest label so adding a row cannot silently break the alignment.
const settingLabelWidth = len("Show dotfolders")

// label is the human-readable name in the first column.
func (s setting) label() string {
	switch s {
	case settingSeek:
		return "Seek step"
	case settingIcons:
		return "Icons"
	case settingShowHidden:
		return "Show dotfolders"
	case settingVolumeStep:
		return "Volume step"
	default:
		return ""
	}
}

// value formats the current value. Seek uses Go duration syntax so the screen
// shows exactly what a hand-edited config file would contain.
func (s setting) value(cfg Config) string {
	switch s {
	case settingSeek:
		return cfg.SeekStep.String()
	case settingIcons:
		return string(cfg.Icons)
	case settingShowHidden:
		if cfg.ShowHidden {
			return "on"
		}
		return "off"
	case settingVolumeStep:
		return fmt.Sprintf("%d%%", cfg.VolumeStep)
	default:
		return ""
	}
}

// openSettings focuses the options screen at the first row. Unsaved changes
// and any save error survive closing and reopening, so a failed write can be
// retried by leaving the screen again.
func (m *Model) openSettings() {
	m.settingsOpen = true
	m.settingsCursor = 0
}

// updateSettings handles keys while the options screen is focused. Closing
// saves through a command; nothing here touches the disk.
func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		// ctrl+c is the escape hatch that always exits, so it does not wait
		// for a save the listener may not want.
		m.quitting = true
		return m, tea.Quit

	case "esc", "q":
		m.settingsOpen = false
		if !m.settingsDirty {
			// Opening and closing without touching anything must not rewrite
			// a file that may have been hand-edited in the meantime.
			return m, nil
		}
		return m, m.saveConfigCmd()

	case "j", "down":
		if m.settingsCursor < int(settingCount)-1 {
			m.settingsCursor++
		}

	case "k", "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}

	case "left":
		m.adjustSetting(-1)

	case "right":
		m.adjustSetting(1)

	case "enter":
		m.activateSetting()
	}

	return m, nil
}

// adjustSetting moves the highlighted value by delta. Seek steps move a second
// at a time and volume steps a percent, because those are the units the config
// file uses; neither can reach zero, which LoadConfig rejects as meaningless.
func (m *Model) adjustSetting(delta int) {
	changed := false

	switch setting(m.settingsCursor) {
	case settingSeek:
		next := max(m.cfg.SeekStep+time.Duration(delta)*time.Second, time.Second)
		if next != m.cfg.SeekStep {
			m.cfg.SeekStep = next
			changed = true
		}

	case settingIcons:
		next := m.cfg.Icons.next(delta)
		if next != m.cfg.Icons {
			m.cfg.Icons = next
			changed = true
		}

	case settingShowHidden:
		// Left and right set the state outright rather than toggling, so the
		// same key always means the same value.
		next := delta > 0
		if next != m.cfg.ShowHidden {
			m.cfg.ShowHidden = next
			changed = true
		}

	case settingVolumeStep:
		next := max(m.cfg.VolumeStep+delta, 1)
		if next != m.cfg.VolumeStep {
			m.cfg.VolumeStep = next
			changed = true
		}
	}

	if changed {
		m.settingsDirty = true
	}
}

// activateSetting applies enter: booleans toggle, everything else advances one
// step the way the right arrow does.
func (m *Model) activateSetting() {
	if setting(m.settingsCursor) != settingShowHidden {
		m.adjustSetting(1)
		return
	}

	m.cfg.ShowHidden = !m.cfg.ShowHidden
	m.settingsDirty = true
}

// settingsView draws the focused options screen. It is a full view rather than
// an overlay so the rows keep the full width; the playlist is hidden while it
// is open and playback continues underneath.
func (m Model) settingsView() string {
	title := "Termelody - options"
	if m.width > 0 {
		title = fitRight(title, m.width)
	}
	heading := titleStyle.Render(title) + "\n\n"

	rows := m.settingsRows()

	help := helpStyle.Render(fitHelp(m.width,
		"  j/k or ↑/↓ move · ←/→ change · enter toggle · esc save & close",
		"  j/k move · ←/→ change\n  enter toggle · esc save & close",
		"  ←/→ change · esc save",
	))

	// The footer is the first part to go when the terminal is short: the
	// values are the reason the screen exists, and Place pads but never
	// clips, so an over-tall block would push the prompt off screen.
	full := heading + strings.Join(rows, "\n") + "\n" + help
	if m.height == 0 || lipgloss.Height(full) <= m.height {
		return m.centerBlock(full)
	}

	body := heading + strings.Join(rows, "\n")
	if lipgloss.Height(body) <= m.height {
		return m.centerBlock(body)
	}

	// The heading no longer fits either; show a window of rows around the
	// cursor so the highlighted preference is always one of them.
	visible := min(m.height, len(rows))
	top := windowTop(m.settingsCursor, len(rows), visible)
	return m.centerBlock(strings.Join(rows[top:top+visible], "\n"))
}

// centerBlock places a block in the middle of the terminal, or returns it
// untouched before the first WindowSizeMsg when there is no size to center in.
func (m Model) centerBlock(block string) string {
	if m.width == 0 || m.height == 0 {
		return block
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}

// settingsRows renders every row as a styled line. The marker, the indent, and
// the style padding spend four cells, so the text itself is fitted to what is
// left and never wraps.
func (m Model) settingsRows() []string {
	rows := make([]string, int(settingCount))
	for row := setting(0); row < settingCount; row++ {
		text := fmt.Sprintf("%-*s  %s", settingLabelWidth, row.label(), row.value(m.cfg))
		if m.width > 0 {
			text = fitRight(text, max(1, m.width-4))
		}

		if row == setting(m.settingsCursor) {
			rows[row] = selectedStyle.Render("> " + text)
		} else {
			rows[row] = trackStyle.Render("  " + text)
		}
	}
	return rows
}
