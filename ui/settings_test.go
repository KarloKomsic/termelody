package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// openSettings opens the options screen and returns the updated Model, so each
// test can start from the same focused state.
func openSettings(t *testing.T, m Model) Model {
	t.Helper()

	m = m.apply(runeMsg('o'))
	if !m.settingsOpen {
		t.Fatal("o did not open the options screen")
	}
	return m
}

// closeSettings sends key, feeds the resulting command through Update, and
// returns the settled model. It is the full close-and-save path a listener
// takes with esc or q.
func closeSettings(t *testing.T, m Model, key tea.KeyMsg) Model {
	t.Helper()

	updated, cmd := m.Update(key)
	m = updated.(Model)
	if cmd != nil {
		m = m.apply(cmd())
	}
	return m
}

func TestOptionsKeyOpensSettings(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width, m.height = 80, 24

	m = openSettings(t, m)
	if m.settingsCursor != 0 {
		t.Errorf("settings cursor = %d, want 0", m.settingsCursor)
	}

	out := m.View()
	if !strings.Contains(out, "Termelody - options") {
		t.Errorf("options view has no title:\n%s", out)
	}
	for _, want := range []string{"Seek step", "Icons", "Show dotfolders", "Volume step"} {
		if !strings.Contains(out, want) {
			t.Errorf("options view is missing %q:\n%s", want, out)
		}
	}
}

func TestSettingsViewShowsCurrentValues(t *testing.T) {
	m := newTestModel(t, Config{
		SeekStep:   10 * time.Second,
		ShowHidden: true,
		Icons:      IconsNerd,
		VolumeStep: 3,
	})
	m.width, m.height = 80, 24

	out := openSettings(t, m).View()
	for _, want := range []string{"10s", "nerd", "on", "3%"} {
		if !strings.Contains(out, want) {
			t.Errorf("options view is missing current value %q:\n%s", want, out)
		}
	}
}

func TestSettingsNavigationStaysInBounds(t *testing.T) {
	m := openSettings(t, newTestModel(t, Config{}))

	for i := 0; i < int(settingCount)+2; i++ {
		m = m.apply(runeMsg('j'))
	}
	if m.settingsCursor != int(settingCount)-1 {
		t.Errorf("settings cursor = %d, want last row %d", m.settingsCursor, int(settingCount)-1)
	}

	for i := 0; i < int(settingCount)+2; i++ {
		m = m.apply(runeMsg('k'))
	}
	if m.settingsCursor != 0 {
		t.Errorf("settings cursor = %d, want first row 0", m.settingsCursor)
	}
}

func TestSettingsAdjustSeek(t *testing.T) {
	m := openSettings(t, newTestModel(t, Config{SeekStep: 5 * time.Second}))

	m = m.apply(keyMsg(tea.KeyRight))
	if want := 6 * time.Second; m.cfg.SeekStep != want {
		t.Errorf("after right: seek = %s, want %s", m.cfg.SeekStep, want)
	}

	m = m.apply(keyMsg(tea.KeyLeft))
	if want := 5 * time.Second; m.cfg.SeekStep != want {
		t.Errorf("after left: seek = %s, want %s", m.cfg.SeekStep, want)
	}

	// A zero-length jump would make the arrows do nothing and the loader
	// rejects it, so the row stops at one second.
	for i := 0; i < 10; i++ {
		m = m.apply(keyMsg(tea.KeyLeft))
	}
	if want := time.Second; m.cfg.SeekStep != want {
		t.Errorf("after holding left: seek = %s, want %s", m.cfg.SeekStep, want)
	}
}

func TestSettingsAdjustIcons(t *testing.T) {
	m := openSettings(t, newTestModel(t, Config{Icons: IconsUnicode}))
	m = m.apply(runeMsg('j'))

	steps := []struct {
		key  tea.KeyMsg
		want IconSet
	}{
		{keyMsg(tea.KeyRight), IconsNerd},
		{keyMsg(tea.KeyRight), IconsNone},
		{keyMsg(tea.KeyRight), IconsUnicode},
		{keyMsg(tea.KeyLeft), IconsNone},
		// enter advances the same way right does.
		{keyMsg(tea.KeyEnter), IconsUnicode},
	}

	for i, s := range steps {
		m = m.apply(s.key)
		if m.cfg.Icons != s.want {
			t.Errorf("step %d: icons = %q, want %q", i+1, m.cfg.Icons, s.want)
		}
	}
}

func TestSettingsAdjustDotfolders(t *testing.T) {
	m := openSettings(t, newTestModel(t, Config{}))
	m = m.apply(runeMsg('j'))
	m = m.apply(runeMsg('j'))

	m = m.apply(keyMsg(tea.KeyRight))
	if !m.cfg.ShowHidden {
		t.Error("right did not enable dotfolders")
	}

	// The same key keeps meaning the same value instead of toggling back.
	m = m.apply(keyMsg(tea.KeyRight))
	if !m.cfg.ShowHidden {
		t.Error("second right disabled dotfolders")
	}

	m = m.apply(keyMsg(tea.KeyLeft))
	if m.cfg.ShowHidden {
		t.Error("left did not disable dotfolders")
	}

	m = m.apply(keyMsg(tea.KeyEnter))
	if !m.cfg.ShowHidden {
		t.Error("enter did not toggle dotfolders on")
	}
}

func TestSettingsAdjustVolumeStep(t *testing.T) {
	m := openSettings(t, newTestModel(t, Config{VolumeStep: 1}))
	m = m.apply(runeMsg('j'))
	m = m.apply(runeMsg('j'))
	m = m.apply(runeMsg('j'))

	m = m.apply(keyMsg(tea.KeyRight))
	if m.cfg.VolumeStep != 2 {
		t.Errorf("after right: volume step = %d, want 2", m.cfg.VolumeStep)
	}

	m = m.apply(keyMsg(tea.KeyLeft))
	if m.cfg.VolumeStep != 1 {
		t.Errorf("after left: volume step = %d, want 1", m.cfg.VolumeStep)
	}

	// One percent is the smallest step that still changes anything.
	m = m.apply(keyMsg(tea.KeyLeft))
	if m.cfg.VolumeStep != 1 {
		t.Errorf("after holding left: volume step = %d, want 1", m.cfg.VolumeStep)
	}
}

func TestSettingsApplyLive(t *testing.T) {
	m := newTestModel(t, Config{
		SeekStep:   5 * time.Second,
		Icons:      IconsUnicode,
		VolumeStep: 1,
	})
	m.width, m.height = 100, 24
	start := m.player.Volume()
	if err := m.player.AdjustVolume(-50); err != nil {
		t.Fatalf("prime volume: %v", err)
	}
	low := max(start-50, 0)
	wantVolume(t, m, low)

	m = openSettings(t, m)
	m = m.apply(keyMsg(tea.KeyRight)) // seek 6s
	m = m.apply(runeMsg('j'))
	m = m.apply(keyMsg(tea.KeyRight)) // nerd
	m = m.apply(runeMsg('j'))
	m = m.apply(runeMsg('j'))
	m = m.apply(keyMsg(tea.KeyRight)) // volume step 2

	// The playlist is hidden while the screen is open, but the values are
	// already the ones the controls will use.
	m.settingsOpen = false
	out := m.View()
	for _, want := range []string{"seek 6s", "\uf001 SNAP! - The Power"} {
		if !strings.Contains(out, want) {
			t.Errorf("playlist view is missing live value %q:\n%s", want, out)
		}
	}

	m = m.apply(runeMsg('+'))
	wantVolume(t, m, low+2)
}

func TestSettingsKeysDoNotReachPlayback(t *testing.T) {
	m := newTestModel(t, Config{})
	m.width, m.height = 80, 24
	m.cursor = 1
	start := m.player.Volume()
	if err := m.player.AdjustVolume(-50); err != nil {
		t.Fatalf("prime volume: %v", err)
	}
	low := max(start-50, 0)
	wantVolume(t, m, low)

	m = openSettings(t, m)
	m = m.apply(runeMsg('n'))
	m = m.apply(runeMsg('p'))
	m = m.apply(runeMsg('s'))
	m = m.apply(runeMsg('+'))
	m = m.apply(runeMsg('G'))

	if m.cursor != 1 {
		t.Errorf("track cursor = %d, want 1: playback keys leaked through settings", m.cursor)
	}
	wantVolume(t, m, low)

	// j moves the settings cursor, not the track cursor.
	m = m.apply(runeMsg('j'))
	if m.settingsCursor != 1 {
		t.Errorf("settings cursor = %d, want 1", m.settingsCursor)
	}
	if m.cursor != 1 {
		t.Errorf("track cursor = %d, want 1 after j in settings", m.cursor)
	}
}

func TestSettingsCloseWithoutChangesDoesNotSave(t *testing.T) {
	path := writeConfig(t, "# hand written\nseek = 5s\n")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	m := newTestModel(t, Config{}).WithConfigPath(path)
	m = openSettings(t, m)

	updated, cmd := m.Update(runeMsg('q'))
	m = updated.(Model)
	if cmd != nil {
		t.Error("closing unchanged settings returned a save command")
	}
	if m.settingsOpen {
		t.Error("q left the options screen open")
	}
	if m.quitting {
		t.Error("q in settings quit the program instead of closing the screen")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("unchanged config was rewritten:\n%s\nwant\n%s", after, before)
	}
}

func TestSettingsSaveOnClose(t *testing.T) {
	path := writeConfig(t, "# hand written\nseek = 5s  # step\n")

	m := newTestModel(t, Config{}).WithConfigPath(path)
	m = openSettings(t, m)
	m = m.apply(keyMsg(tea.KeyRight))

	updated, cmd := m.Update(keyMsg(tea.KeyEsc))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("closing changed settings returned no save command")
	}
	if m.quitting {
		t.Error("esc quit the program instead of closing the screen")
	}

	// The dirty flag clears only after the write reports back, not when the
	// key is handled.
	if !m.settingsDirty {
		t.Error("settings were marked saved before the write ran")
	}
	m = m.apply(cmd())
	if m.settingsDirty {
		t.Error("settings stayed dirty after a successful save")
	}
	if m.configErr != nil {
		t.Errorf("config error after save: %v", m.configErr)
	}

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if want := 6 * time.Second; got.SeekStep != want {
		t.Errorf("saved seek = %s, want %s", got.SeekStep, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, want := range []string{"# hand written", "# step"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved config lost %q:\n%s", want, data)
		}
	}
}

func TestSettingsSaveFailureIsReportedAndRetried(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	m := newTestModel(t, Config{}).WithConfigPath(filepath.Join(blocker, "config"))
	m.width, m.height = 100, 24
	m = openSettings(t, m)
	m = m.apply(keyMsg(tea.KeyRight))

	updated, cmd := m.Update(runeMsg('q'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("closing changed settings returned no save command")
	}
	m = m.apply(cmd())

	if m.configErr == nil {
		t.Fatal("a failed save reported no error")
	}
	if !m.settingsDirty {
		t.Error("a failed save cleared the dirty flag, so the write would never be retried")
	}
	if out := m.View(); !strings.Contains(out, "could not save settings") {
		t.Errorf("view does not report the failed save:\n%s", out)
	}

	// Once the path is usable, closing the still-dirty screen retries and
	// clears the error.
	good := filepath.Join(t.TempDir(), "config")
	m.configPath = good
	m = m.apply(runeMsg('o'))

	updated, cmd = m.Update(runeMsg('q'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("reopening and closing did not retry the failed save")
	}
	m = m.apply(cmd())

	if m.configErr != nil {
		t.Errorf("config error after a successful retry: %v", m.configErr)
	}
	if m.settingsDirty {
		t.Error("settings stayed dirty after a successful retry")
	}
	if _, err := LoadConfig(good); err != nil {
		t.Errorf("retried config does not load: %v", err)
	}
}

func TestSettingsKeepsDirtyWhenEditedDuringSave(t *testing.T) {
	path := writeConfig(t, "seek = 5s\n")

	m := newTestModel(t, Config{}).WithConfigPath(path)
	m = openSettings(t, m)
	m = m.apply(keyMsg(tea.KeyRight))

	updated, cmd := m.Update(keyMsg(tea.KeyEsc))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("closing changed settings returned no save command")
	}

	// Reopen and edit before the first write reports back: its snapshot is
	// now stale, so the success must not mark the newer value as saved.
	m = openSettings(t, m)
	m = m.apply(keyMsg(tea.KeyRight))

	m = m.apply(cmd())
	if !m.settingsDirty {
		t.Error("a stale successful save cleared the dirty flag for a newer edit")
	}

	updated, cmd = m.Update(runeMsg('q'))
	m = updated.(Model)
	m = m.apply(cmd())

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if want := 7 * time.Second; got.SeekStep != want {
		t.Errorf("saved seek = %s, want %s", got.SeekStep, want)
	}
	if m.settingsDirty {
		t.Error("settings stayed dirty after the newer value was saved")
	}
}

func TestSettingsCtrlCQuits(t *testing.T) {
	path := writeConfig(t, "seek = 5s\n")
	m := newTestModel(t, Config{}).WithConfigPath(path)
	m = openSettings(t, m)
	m = m.apply(keyMsg(tea.KeyRight))

	updated, cmd := m.Update(keyMsg(tea.KeyCtrlC))
	m = updated.(Model)

	if !m.quitting {
		t.Error("ctrl+c in settings did not quit")
	}
	if cmd == nil {
		t.Error("ctrl+c in settings returned no quit command")
	}
	if out := m.View(); out != "" {
		t.Errorf("quitting view = %q, want empty", out)
	}
}

func TestSettingsViewFitsTheTerminal(t *testing.T) {
	sizes := []struct {
		width, height int
	}{
		{80, 24},
		{40, 20},
		{30, 10},
		{20, 4},
	}

	for _, size := range sizes {
		m := openSettings(t, newTestModel(t, Config{}))
		m.width, m.height = size.width, size.height

		out := m.View()
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) > size.height {
			t.Errorf("%dx%d: view is %d lines, want at most %d", size.width, size.height, len(lines), size.height)
		}
		for i, line := range lines {
			if got := lipgloss.Width(line); got > size.width {
				t.Errorf("%dx%d: line %d is %d wide: %q", size.width, size.height, i+1, got, line)
			}
		}
	}
}
