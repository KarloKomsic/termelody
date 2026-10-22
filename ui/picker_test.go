package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// newTestPicker builds Music/Album and Videos/Clips plus a stray audio file,
// then returns a picker on that root with its listing already loaded.
func newTestPicker(t *testing.T) (string, Picker) {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{".config/nvim", ".local/share", "Music/Album", "Videos/Clips"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "song.mp3"), nil, 0o644); err != nil {
		t.Fatalf("creating file: %v", err)
	}

	m := NewPicker(root, Config{})
	m = applyPickerMsg(t, m, m.Init())

	return root, m
}

// apply feeds a message through Update and returns both results, since a
// navigation key can schedule a directory read.
func (m Picker) apply(msg tea.Msg) (Picker, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(Picker), cmd
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()

	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	return cmd()
}

func applyPickerMsg(t *testing.T, m Picker, cmd tea.Cmd) Picker {
	t.Helper()

	updated, _ := m.apply(runCmd(t, cmd))
	return updated
}

func TestPickerListsOnlyDirectories(t *testing.T) {
	_, m := newTestPicker(t)

	// os.ReadDir sorts by name, so the order is predictable without a sort
	// of our own. Dot-directories exist in the tree but stay hidden by
	// default.
	if got := strings.Join(m.entries, ","); got != "Music,Videos" {
		t.Errorf("entries = %q, want %q", got, "Music,Videos")
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestPickerToggleShowsDotDirectories(t *testing.T) {
	_, m := newTestPicker(t)

	m, _ = m.apply(runeMsg('.'))
	if got := strings.Join(m.entries, ","); got != ".config,.local,Music,Videos" {
		t.Errorf("after toggle: entries = %q, want dot-directories too", got)
	}
	// Music was selected before the toggle and must remain selected after it.
	if m.cursor != 2 {
		t.Errorf("after toggle: cursor = %d, want 2 (Music)", m.cursor)
	}

	m, _ = m.apply(runeMsg('.'))
	if got := strings.Join(m.entries, ","); got != "Music,Videos" {
		t.Errorf("after second toggle: entries = %q, want dot-directories hidden again", got)
	}
	if m.cursor != 0 {
		t.Errorf("after second toggle: cursor = %d, want 0 (Music)", m.cursor)
	}
}

func TestPickerToggleHidingSelectedDotDirectoryFallsBackToTop(t *testing.T) {
	_, m := newTestPicker(t)

	m, _ = m.apply(runeMsg('.'))
	m, _ = m.apply(runeMsg('g'))
	if m.entries[m.cursor] != ".config" {
		t.Fatalf("cursor = %q, want .config", m.entries[m.cursor])
	}

	m, _ = m.apply(runeMsg('.'))

	if len(m.entries) == 0 || m.entries[m.cursor] != "Music" {
		t.Errorf("cursor = %q, want the first visible folder after hiding", m.entries[m.cursor])
	}
	if strings.Contains(strings.Join(m.entries, ","), ".config") {
		t.Errorf("entries = %q, want dot-directories hidden", strings.Join(m.entries, ","))
	}
}

func TestPickerConfigShowsDotDirectoriesFromStart(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Music"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewPicker(root, Config{ShowHidden: true})
	m = applyPickerMsg(t, m, m.Init())

	if got := strings.Join(m.entries, ","); got != ".config,Music" {
		t.Errorf("entries = %q, want %q", got, ".config,Music")
	}
}

func TestPickerListsEntersAndSelectsSymlinkedDirectory(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, "Album"), 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "Linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	m := NewPicker(root, Config{})
	m = applyPickerMsg(t, m, m.Init())

	if got := strings.Join(m.entries, ","); got != "Linked" {
		t.Fatalf("entries = %q, want %q", got, "Linked")
	}
	if !m.links["Linked"] {
		t.Error("symlinked directory was not recorded as a link")
	}

	m.width, m.height = 100, 24
	if !strings.Contains(m.View(), "Linked@") {
		t.Errorf("view does not mark the symlink with @:\n%s", m.View())
	}

	m, cmd := m.apply(keyMsg(tea.KeyEnter))
	m = applyPickerMsg(t, m, cmd)

	// The link path is kept rather than the resolved target, so backing out
	// returns to where the listener came from.
	if m.dir != link {
		t.Fatalf("dir = %q, want the link path %q", m.dir, link)
	}
	if got := strings.Join(m.entries, ","); got != "Album" {
		t.Errorf("entries = %q, want %q", got, "Album")
	}
	if m.links["Album"] {
		t.Error("a real directory inside the target was marked as a link")
	}
	if !strings.Contains(m.View(), "Album/") {
		t.Errorf("view does not use the directory marker for a real folder:\n%s", m.View())
	}

	m, cmd = m.apply(runeMsg('h'))
	m = applyPickerMsg(t, m, cmd)

	if m.dir != root {
		t.Fatalf("after back: dir = %q, want %q", m.dir, root)
	}
	if len(m.entries) == 0 || m.entries[m.cursor] != "Linked" {
		t.Errorf("cursor lost the link after backing out: entries %q, cursor %d", m.entries, m.cursor)
	}

	m, cmd = m.apply(runeMsg('s'))
	if m.Chosen() != link {
		t.Errorf("Chosen() = %q, want %q", m.Chosen(), link)
	}
	if _, ok := runCmd(t, cmd).(tea.QuitMsg); !ok {
		t.Error("selecting should quit the picker")
	}
}

func TestPickerSkipsBrokenAndFileSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Real"), 0o755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(root, "song.mp3")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "Broken")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(file, filepath.Join(root, "FileLink")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	m := NewPicker(root, Config{})
	m = applyPickerMsg(t, m, m.Init())

	if got := strings.Join(m.entries, ","); got != "Real" {
		t.Errorf("entries = %q, want only the real directory", got)
	}
}

func TestPickerCursorStopsAtBothEnds(t *testing.T) {
	_, m := newTestPicker(t)

	// Two downward presses against two entries must land on the last one
	// rather than wrapping or running out of bounds.
	m, _ = m.apply(runeMsg('j'))
	m, _ = m.apply(keyMsg(tea.KeyDown))
	if m.cursor != 1 {
		t.Errorf("after two downward keys: cursor = %d, want 1", m.cursor)
	}

	m, _ = m.apply(runeMsg('k'))
	m, _ = m.apply(keyMsg(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("after two upward keys: cursor = %d, want 0", m.cursor)
	}

	m, _ = m.apply(runeMsg('G'))
	if m.cursor != 1 {
		t.Errorf("G: cursor = %d, want 1", m.cursor)
	}

	m, _ = m.apply(runeMsg('g'))
	if m.cursor != 0 {
		t.Errorf("g: cursor = %d, want 0", m.cursor)
	}
}

func TestPickerDescendAndBackKeepsPlace(t *testing.T) {
	root, m := newTestPicker(t)

	m, _ = m.apply(runeMsg('j'))
	m, cmd := m.apply(keyMsg(tea.KeyEnter))
	m = applyPickerMsg(t, m, cmd)

	if want := filepath.Join(root, "Videos"); m.dir != want {
		t.Fatalf("after enter: dir = %q, want %q", m.dir, want)
	}
	if got := strings.Join(m.entries, ","); got != "Clips" {
		t.Errorf("entries = %q, want %q", got, "Clips")
	}

	m, cmd = m.apply(runeMsg('h'))
	m = applyPickerMsg(t, m, cmd)

	if m.dir != root {
		t.Errorf("after back: dir = %q, want %q", m.dir, root)
	}
	if m.cursor != 1 {
		t.Errorf("after back: cursor = %d, want 1: the folder left should stay selected", m.cursor)
	}
}

func TestPickerBackAliases(t *testing.T) {
	for _, key := range []tea.KeyMsg{runeMsg('h'), keyMsg(tea.KeyLeft), keyMsg(tea.KeyBackspace)} {
		root, m := newTestPicker(t)
		m, _ = m.apply(runeMsg('j'))

		m, cmd := m.apply(keyMsg(tea.KeyEnter))
		m = applyPickerMsg(t, m, cmd)

		m, cmd = m.apply(key)
		m = applyPickerMsg(t, m, cmd)

		if m.dir != root {
			t.Errorf("%s: dir = %q, want %q", key.String(), m.dir, root)
		}
	}
}

func TestPickerOpenAliases(t *testing.T) {
	for _, key := range []tea.KeyMsg{keyMsg(tea.KeyEnter), keyMsg(tea.KeyRight), runeMsg('l')} {
		root, m := newTestPicker(t)
		m, _ = m.apply(runeMsg('j'))

		m, cmd := m.apply(key)
		m = applyPickerMsg(t, m, cmd)

		if want := filepath.Join(root, "Videos"); m.dir != want {
			t.Errorf("%s: dir = %q, want %q", key.String(), m.dir, want)
		}
	}
}

func TestPickerRootHasNoParent(t *testing.T) {
	m := Picker{dir: string(filepath.Separator)}

	_, cmd := m.apply(runeMsg('h'))
	if cmd != nil {
		t.Error("backing out of the filesystem root should do nothing")
	}
}

func TestPickerSelectScansHighlightedDirectory(t *testing.T) {
	root, m := newTestPicker(t)

	// The cursor starts on Music, so that is what s must choose -- not the
	// root the picker happens to be browsing.
	m, cmd := m.apply(runeMsg('s'))

	want := filepath.Join(root, "Music")
	if m.Chosen() != want {
		t.Errorf("Chosen() = %q, want %q", m.Chosen(), want)
	}
	if m.Canceled() {
		t.Error("selecting a directory must not count as canceling")
	}
	if _, ok := runCmd(t, cmd).(tea.QuitMsg); !ok {
		t.Error("selecting should quit the picker")
	}
}

func TestPickerSpaceScansCurrentDirectory(t *testing.T) {
	root, m := newTestPicker(t)

	m, cmd := m.apply(keyMsg(tea.KeySpace))

	if m.Chosen() != root {
		t.Errorf("Chosen() = %q, want %q", m.Chosen(), root)
	}
	if _, ok := runCmd(t, cmd).(tea.QuitMsg); !ok {
		t.Error("selecting should quit the picker")
	}
}

func TestPickerSelectDoesNothingWithoutEntries(t *testing.T) {
	m := NewPicker("/music", Config{})

	m, cmd := m.apply(runeMsg('s'))

	if cmd != nil {
		t.Error("s with nothing highlighted should not quit")
	}
	if m.Chosen() != "" {
		t.Errorf("Chosen() = %q, want empty", m.Chosen())
	}
}

func TestPickerCancelQuits(t *testing.T) {
	for _, key := range []tea.KeyMsg{runeMsg('q'), keyMsg(tea.KeyEsc), keyMsg(tea.KeyCtrlC)} {
		m := NewPicker("/music", Config{})

		m, cmd := m.apply(key)

		if !m.Canceled() {
			t.Errorf("%s: Canceled() = false, want true", key.String())
		}
		if m.Chosen() != "" {
			t.Errorf("%s: Chosen() = %q, want empty", key.String(), m.Chosen())
		}
		if _, ok := runCmd(t, cmd).(tea.QuitMsg); !ok {
			t.Errorf("%s: canceling should quit the picker", key.String())
		}
	}
}

func TestPickerKeepsLocationWhenReadFails(t *testing.T) {
	root, m := newTestPicker(t)

	m = applyPickerMsg(t, m, loadDir(filepath.Join(root, "missing")))

	if m.err == nil {
		t.Fatal("expected an error for a missing directory")
	}
	if m.dir != root {
		t.Errorf("dir = %q, want %q: a failed read must not move the picker", m.dir, root)
	}
	if got := strings.Join(m.entries, ","); got != "Music,Videos" {
		t.Errorf("entries = %q, want the previous listing", got)
	}
}

func TestPickerViewFitsEightyColumns(t *testing.T) {
	_, m := newTestPicker(t)
	m, _ = m.apply(tea.WindowSizeMsg{Width: 80, Height: 24})

	out := m.View()
	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line %d is %d wide, overflows 80: %q", i+1, w, line)
		}
	}
}

func TestPickerViewFitsNarrowTerminal(t *testing.T) {
	_, m := newTestPicker(t)
	m, _ = m.apply(tea.WindowSizeMsg{Width: 40, Height: 20})

	out := m.View()
	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("line %d is %d wide, overflows 40: %q", i+1, w, line)
		}
	}

	// The narrow help keeps the quit key on screen instead of clipping it.
	if !strings.Contains(out, "q quit") {
		t.Errorf("narrow view lost the quit hint:\n%s", out)
	}
	// The temp root is longer than the line, so the path must be shortened
	// from the left rather than losing the folder being browsed.
	if !strings.Contains(out, "…") {
		t.Errorf("long path was not shortened:\n%s", out)
	}
}

func TestPickerNoticeSurvivesInitialLoadAndClearsOnNavigation(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"Next", "Other"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	const notice = "No tracks found in /old"
	m := NewPicker(root, Config{}).WithNotice(notice)
	m = applyPickerMsg(t, m, m.Init())
	m.width, m.height = 100, 24

	// Reopening on the rejected folder reads that same folder, which must not
	// clear the reason it was reopened.
	if !strings.Contains(m.View(), notice) {
		t.Errorf("view does not show the notice:\n%s", m.View())
	}

	m, cmd := m.apply(keyMsg(tea.KeyEnter))
	m = applyPickerMsg(t, m, cmd)

	if strings.Contains(m.View(), notice) {
		t.Errorf("notice survived navigating elsewhere:\n%s", m.View())
	}
}

func TestPickerViewShowsPathSelectionAndError(t *testing.T) {
	root, m := newTestPicker(t)
	m.width, m.height = 80, 24

	out := m.View()
	if !strings.Contains(out, root) {
		t.Errorf("view does not show the current directory:\n%s", out)
	}
	if !strings.Contains(out, "> Music/") {
		t.Errorf("view does not mark the selected folder:\n%s", out)
	}
	if !strings.Contains(out, "s scan selected") {
		t.Errorf("view does not document the select key:\n%s", out)
	}

	missing := filepath.Join(root, "missing")
	m = applyPickerMsg(t, m, loadDir(missing))

	out = m.View()
	if !strings.Contains(out, missing) {
		t.Errorf("view does not show the read error:\n%s", out)
	}
	if strings.Contains(out, "(no folders here)") {
		t.Errorf("view shows the empty placeholder while an error explains it:\n%s", out)
	}
}
