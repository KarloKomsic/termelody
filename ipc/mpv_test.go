package ipc

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// testFile creates a short silent WAV so tests do not depend on the gitignored
// music/ directory. Returns an empty string if the file cannot be created, in
// which case the caller should skip.
func testFile(t *testing.T, seconds int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "tone.wav")

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return ""
	}

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
		"-t", itoa(seconds), "-y", path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg could not build a fixture: %v: %s", err, out)
	}

	return path
}

func itoa(n int) string { return strconv.Itoa(n) }

func newTestMPV(t *testing.T) *MPV {
	t.Helper()

	sock := filepath.Join(t.TempDir(), "mpv.sock")

	m, err := LaunchEmpty(sock)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}

	t.Cleanup(func() { m.terminate() })
	return m
}

// awaitEvent reads events until one with the wanted name arrives.
func awaitEvent(t *testing.T, m *MPV, want string, timeout time.Duration) Event {
	t.Helper()
	return awaitEventWhere(t, m, want, nil, timeout)
}

// awaitEventWhere reads events until one with the wanted name satisfies match.
// Observing a property makes mpv report its current value straight away, so
// callers that care about a specific change must filter rather than take the
// first event of that name.
func awaitEventWhere(t *testing.T, m *MPV, want string, match func(Event) bool, timeout time.Duration) Event {
	t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-m.Events():
			if !ok {
				t.Fatalf("event channel closed before %q", want)
			}
			if ev.Name != want {
				continue
			}
			if match == nil || match(ev) {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

// decodeChange reads a property-change event.
func decodeChange(t *testing.T, ev Event) (name string, data bool) {
	t.Helper()

	var change struct {
		Name string `json:"name"`
		Data bool   `json:"data"`
	}
	if err := json.Unmarshal(ev.Raw, &change); err != nil {
		t.Fatalf("property-change payload: %v", err)
	}
	return change.Name, change.Data
}

func TestCommandReturnsReplyNotEvent(t *testing.T) {
	m := newTestMPV(t)

	// With events being pushed to every connection, a naive read can pick up
	// an event instead of the reply. Repeat to make the race likely.
	for range 20 {
		data, err := m.Command([]any{"get_property", "pause"})
		if err != nil {
			t.Fatalf("get_property: %v", err)
		}
		if _, ok := data.(bool); !ok {
			t.Fatalf("data = %#v, want bool", data)
		}
	}
}

func TestCommandSkipsInterleavedEvents(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	m := newTestMPV(t)
	if err := m.Play(file); err != nil {
		t.Fatalf("play: %v", err)
	}

	// Playing generates a burst of events (start-file, file-loaded, ...) that
	// land on the same connection the commands use.
	for range 30 {
		if _, err := m.Command([]any{"get_property", "pause"}); err != nil {
			t.Fatalf("get_property during event burst: %v", err)
		}
	}
}

func TestEventsArrive(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	// mpv is started idle and the file is loaded afterwards, so the listener
	// is already attached and cannot miss the start of playback.
	m := newTestMPV(t)
	if err := m.Play(file); err != nil {
		t.Fatalf("play: %v", err)
	}

	awaitEvent(t, m, "file-loaded", 10*time.Second)
}

func TestEndFileReportsEOFReason(t *testing.T) {
	file := testFile(t, 1)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	m := newTestMPV(t)
	if err := m.Play(file); err != nil {
		t.Fatalf("play: %v", err)
	}

	// The fixture is 1s and mpv is idle, so it survives to report the end.
	ev := awaitEvent(t, m, "end-file", 20*time.Second)

	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(ev.Raw, &payload); err != nil {
		t.Fatalf("end-file payload: %v", err)
	}
	if payload.Reason != "eof" {
		t.Errorf("reason = %q, want eof", payload.Reason)
	}
}

// Auto-advance keys off reason == "eof", so the reason reported for a file
// that was interrupted rather than finished is what decides whether that
// check is safe at all. If this ever reports eof, pressing n would skip a
// track.
func TestInterruptedFileIsNotEOF(t *testing.T) {
	long := testFile(t, 30)
	other := testFile(t, 30)
	if long == "" || other == "" {
		t.Skip("ffmpeg unavailable")
	}

	m := newTestMPV(t)
	if err := m.Play(long); err != nil {
		t.Fatalf("play: %v", err)
	}
	// Waiting for file-loaded means the first file is genuinely playing, so
	// the load below interrupts it instead of replacing an empty player.
	awaitEvent(t, m, "file-loaded", 10*time.Second)

	if err := m.Play(other); err != nil {
		t.Fatalf("play replacement: %v", err)
	}

	ev := awaitEvent(t, m, "end-file", 10*time.Second)

	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(ev.Raw, &payload); err != nil {
		t.Fatalf("end-file payload: %v", err)
	}
	if payload.Reason == "eof" {
		t.Errorf("reason = %q, want anything but eof: an interrupted file is not a finished one", payload.Reason)
	}
}

func TestEventsChannelClosesWhenMPVExits(t *testing.T) {
	m := newTestMPV(t)

	if err := m.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}

	// Closing the channel is what lets consumers range without a leak.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-m.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("event channel was not closed after quit")
		}
	}
}

func TestDoneClosesAndErrIsReadableAfterExit(t *testing.T) {
	m := newTestMPV(t)

	if err := m.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}

	select {
	case <-m.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done was not closed after quit")
	}

	// Err is only safe to call after Done; it must not block or panic.
	_ = m.Err()
}

func TestObservePropertyReportsChanges(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	m := newTestMPV(t)
	if err := m.Play(file); err != nil {
		t.Fatalf("play: %v", err)
	}

	// Observation only takes effect on the connection that requests it, so
	// this fails unless ObserveProperty writes to the listener.
	if err := m.ObserveProperty("pause"); err != nil {
		t.Fatalf("observe: %v", err)
	}

	if err := m.Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}

	ev := awaitEventWhere(t, m, "property-change",
		func(ev Event) bool {
			name, data := decodeChange(t, ev)
			return name == "pause" && data
		}, 10*time.Second)

	name, data := decodeChange(t, ev)
	if name != "pause" {
		t.Errorf("name = %q, want pause", name)
	}
	if !data {
		t.Error("data = false, want true after pausing")
	}
}

func TestObservePropertyReportsUnpauseToo(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	m := newTestMPV(t)
	if err := m.Play(file); err != nil {
		t.Fatalf("play: %v", err)
	}
	if err := m.ObserveProperty("pause"); err != nil {
		t.Fatalf("observe: %v", err)
	}

	// Both directions must be reported, so that a toggle is observable.
	if err := m.TogglePause(); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	awaitEventWhere(t, m, "property-change", func(ev Event) bool {
		_, data := decodeChange(t, ev)
		return data
	}, 10*time.Second)

	if err := m.TogglePause(); err != nil {
		t.Fatalf("toggle off: %v", err)
	}
	awaitEventWhere(t, m, "property-change", func(ev Event) bool {
		_, data := decodeChange(t, ev)
		return !data
	}, 10*time.Second)
}

func TestObservePropertyFailsWithoutListener(t *testing.T) {
	// NewMPV never opens a listener, so observing must report the problem
	// rather than silently doing nothing.
	m := NewMPV(filepath.Join(t.TempDir(), "mpv.sock"))
	if err := m.ObserveProperty("pause"); err == nil {
		t.Error("expected an error when observing without a listener")
	}
}

func TestSocketRemovedOnExit(t *testing.T) {
	m := newTestMPV(t)
	sock := m.socketPath

	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("socket should exist while running: %v", err)
	}

	if err := m.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	<-m.Done()

	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Errorf("socket should be removed on exit, stat err = %v", err)
	}
}

func TestCommandFailsCleanlyAfterExit(t *testing.T) {
	m := newTestMPV(t)
	if err := m.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	<-m.Done()

	// Must return an error rather than hang or panic.
	if _, err := m.Command([]any{"get_property", "pause"}); err == nil {
		t.Error("expected an error when talking to a dead mpv")
	}
}

func TestDefaultSocketPathIsPrivateAndUnique(t *testing.T) {
	pathA, dirA, err := resolveSocketPath("")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	t.Cleanup(func() { cleanupSocketDir(dirA) })

	pathB, dirB, err := resolveSocketPath("")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	t.Cleanup(func() { cleanupSocketDir(dirB) })

	if dirA == dirB {
		t.Errorf("two instances shared directory %q", dirA)
	}
	if pathA == pathB {
		t.Errorf("two instances shared socket path %q", pathA)
	}
	if filepath.Dir(pathA) != dirA {
		t.Errorf("socket %q is not inside its own directory %q", pathA, dirA)
	}

	// Anyone able to write the socket can drive mpv, so the directory that
	// holds it must not be reachable by other users.
	info, err := os.Stat(dirA)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("directory mode = %04o, want no group or other access", perm)
	}
}

func TestDefaultSocketDirIsRemovedOnQuit(t *testing.T) {
	m, err := LaunchEmpty("")
	if err != nil {
		t.Fatalf("launch: %v", err)
	}

	if m.socketDir == "" {
		t.Fatal("no private directory was created")
	}
	if _, err := os.Stat(filepath.Join(m.socketDir, "mpv.sock")); err != nil {
		t.Fatalf("socket not created: %v", err)
	}

	if err := m.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}

	if _, err := os.Stat(m.socketDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("private directory survived quit: %v", err)
	}
}

// within runs fn and fails if it does not return promptly, so that a blocking
// mistake surfaces as a test failure rather than a hung suite.
func within(t *testing.T, limit time.Duration, fn func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()

	select {
	case <-done:
	case <-time.After(limit):
		t.Fatalf("did not finish within %s, so it is almost certainly blocking", limit)
	}
}

func TestQuitOnExternallyOwnedClientReturnsInsteadOfPanicking(t *testing.T) {
	// Nothing is listening, so the command fails. Quit then reaches
	// terminate, which used to dereference the nil cmd and panic.
	m := NewMPV(filepath.Join(t.TempDir(), "nobody.sock"))

	var err error
	within(t, 5*time.Second, func() { err = m.Quit() })

	if err == nil {
		t.Error("expected an error when nobody is listening")
	}
}

func TestErrOnExternallyOwnedClientDoesNotBlock(t *testing.T) {
	m := NewMPV(filepath.Join(t.TempDir(), "nobody.sock"))

	within(t, 5*time.Second, func() {
		if err := m.Err(); err != nil {
			t.Errorf("Err = %v, want nil", err)
		}
	})
}

func TestExternallyOwnedQuitEndsTheOwnedProcess(t *testing.T) {
	owned, err := LaunchEmpty("")
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	t.Cleanup(func() { _ = owned.Quit() })

	// A second client attaches to the same mpv without owning it, which is
	// what NewMPV is for. Quitting through it must still work end to end.
	external := NewMPV(owned.socketPath)
	if err := external.Quit(); err != nil {
		t.Fatalf("external quit: %v", err)
	}

	select {
	case <-owned.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the owning client never saw mpv exit")
	}
}

func TestPauseIsResetWhenTheNextTrackLoads(t *testing.T) {
	pausedFile := testFile(t, 30)
	nextFile := testFile(t, 30)
	if pausedFile == "" || nextFile == "" {
		t.Skip("ffmpeg unavailable")
	}

	m, err := LaunchEmpty(filepath.Join(t.TempDir(), "s.sock"))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer m.Quit()

	if err := m.ObserveProperty("pause"); err != nil {
		t.Fatalf("observe: %v", err)
	}

	if err := m.Play(pausedFile); err != nil {
		t.Fatalf("play first: %v", err)
	}
	// The reset applies at playback start, so the file has to be loaded
	// before pausing, otherwise the load would clear it again.
	awaitEvent(t, m, "file-loaded", 10*time.Second)

	if err := m.Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}
	awaitEventWhere(t, m, "property-change", func(ev Event) bool {
		name, data := decodeChange(t, ev)
		return name == "pause" && data
	}, 10*time.Second)

	// Loading the next track must clear the pause, otherwise the new track
	// silently starts paused and nothing in the UI would explain why.
	if err := m.Play(nextFile); err != nil {
		t.Fatalf("play next: %v", err)
	}
	awaitEventWhere(t, m, "property-change", func(ev Event) bool {
		name, data := decodeChange(t, ev)
		return name == "pause" && !data
	}, 10*time.Second)

	got, err := m.GetProperty("pause")
	if err != nil {
		t.Fatalf("get pause: %v", err)
	}
	if got != false {
		t.Errorf("pause = %v, want false after loading the next track", got)
	}
}

// decodeNumberChange reads a property-change whose payload is a number, which
// decodeChange cannot do because it decodes a boolean.
func decodeNumberChange(t *testing.T, ev Event) (name string, value float64) {
	t.Helper()

	var change struct {
		Name string          `json:"name"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(ev.Raw, &change); err != nil {
		t.Fatalf("property-change payload: %v", err)
	}
	if err := json.Unmarshal(change.Data, &value); err != nil {
		t.Fatalf("property-change data: %v", err)
	}
	return change.Name, value
}

// Asking for a property makes mpv report its current value before it reports
// any change, so both reports arrive as the same event name and have to be
// told apart by reading the value rather than by waiting for a shape.
func TestObservePropertyReportsVolume(t *testing.T) {
	m := newTestMPV(t)

	if err := m.SetProperty("volume", 60); err != nil {
		t.Fatalf("set volume: %v", err)
	}

	// Observation is scoped to the connection that requests it, so this fails
	// unless ObserveProperty writes to the listener rather than the socket.
	if err := m.ObserveProperty("volume"); err != nil {
		t.Fatalf("observe: %v", err)
	}

	awaitEventWhere(t, m, "property-change", func(ev Event) bool {
		name, value := decodeNumberChange(t, ev)
		return name == "volume" && value == 60
	}, 10*time.Second)

	if err := m.AddToProperty("volume", -10); err != nil {
		t.Fatalf("add: %v", err)
	}

	awaitEventWhere(t, m, "property-change", func(ev Event) bool {
		name, value := decodeNumberChange(t, ev)
		return name == "volume" && value == 50
	}, 10*time.Second)
}

// The value is asked back rather than watched for, so this holds down the
// arithmetic itself rather than the reporting of it.
func TestAddToPropertyMovesTheValue(t *testing.T) {
	m := newTestMPV(t)

	if err := m.SetProperty("volume", 60); err != nil {
		t.Fatalf("set volume: %v", err)
	}

	cases := []struct {
		delta float64
		want  float64
	}{
		{-10, 50},
		{25, 75},
		{0, 75},
	}

	for _, c := range cases {
		if err := m.AddToProperty("volume", c.delta); err != nil {
			t.Fatalf("add %v: %v", c.delta, err)
		}

		raw, err := m.GetProperty("volume")
		if err != nil {
			t.Fatalf("get volume: %v", err)
		}
		got, ok := raw.(float64)
		if !ok {
			t.Fatalf("volume came back as %T, want a number", raw)
		}
		if got != c.want {
			t.Errorf("volume after add %v = %v, want %v", c.delta, got, c.want)
		}
	}
}
