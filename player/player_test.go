package player

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"codeberg.org/karlokomsic/termelody/playlist"
)

// testFile builds a short silent WAV so tests do not depend on the gitignored
// music/ directory. Returns "" when it cannot be built, meaning skip.
func testFile(t *testing.T, seconds int) string {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return ""
	}

	path := filepath.Join(t.TempDir(), "tone.wav")

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
		"-t", strconv.Itoa(seconds), "-y", path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg could not build a fixture: %v: %s", err, out)
	}

	return path
}

// tracks builds a playlist from raw paths.
func tracks(paths ...string) []playlist.Track {
	out := make([]playlist.Track, 0, len(paths))
	for _, p := range paths {
		out = append(out, playlist.Track{Path: p})
	}
	return out
}

func newTestPlayer(t *testing.T) *Player {
	t.Helper()

	p, err := NewWithConfig(Config{SocketPath: filepath.Join(t.TempDir(), "mpv.sock")})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	t.Cleanup(func() { p.Close() })
	return p
}

// wantState polls until the player reports the wanted state, since state is
// driven by events that arrive shortly after a command.
func wantState(t *testing.T, p *Player, want State) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if p.State() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("state = %s, want %s", p.State(), want)
}

func TestStartsStopped(t *testing.T) {
	p := newTestPlayer(t)
	if got := p.State(); got != StateStopped {
		t.Errorf("state = %s, want stopped", got)
	}
}

func TestStateFollowsEventsNotCommands(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	p := newTestPlayer(t)
	p.SetPlaylist(tracks(file))

	if err := p.PlayIndex(0); err != nil {
		t.Fatalf("play: %v", err)
	}
	wantState(t, p, StatePlaying)

	if err := p.Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}
	wantState(t, p, StatePaused)

	if err := p.Resume(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	wantState(t, p, StatePlaying)

	if err := p.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	wantState(t, p, StateStopped)
}

func TestStateReturnsToStoppedWhenTrackEnds(t *testing.T) {
	file := testFile(t, 1)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	p := newTestPlayer(t)
	p.SetPlaylist(tracks(file))

	if err := p.PlayIndex(0); err != nil {
		t.Fatalf("play: %v", err)
	}
	wantState(t, p, StatePlaying)

	// The fixture is 1s; nobody tells the player it ended, so the end-file
	// event is the only thing that can reveal it.
	wantState(t, p, StateStopped)
}

func TestTogglePauseTracksReality(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	p := newTestPlayer(t)
	p.SetPlaylist(tracks(file))
	if err := p.PlayIndex(0); err != nil {
		t.Fatalf("play: %v", err)
	}
	wantState(t, p, StatePlaying)

	// Toggle must land on paused even though the caller never says which
	// direction it was heading; mpv reports the outcome.
	if err := p.TogglePause(); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	wantState(t, p, StatePaused)

	if err := p.TogglePause(); err != nil {
		t.Fatalf("toggle back: %v", err)
	}
	wantState(t, p, StatePlaying)
}

func TestSeekIsNoopWhenNothingLoaded(t *testing.T) {
	p := newTestPlayer(t)

	// Must not error, and must not be mistaken for a loaded file.
	if err := p.Seek(5 * time.Second); err != nil {
		t.Errorf("seek while stopped: %v", err)
	}
	if got := p.State(); got != StateStopped {
		t.Errorf("state = %s, want stopped", got)
	}
}

func TestEventsAreForwarded(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	p := newTestPlayer(t)
	p.SetPlaylist(tracks(file))
	if err := p.PlayIndex(0); err != nil {
		t.Fatalf("play: %v", err)
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-p.Events():
			if !ok {
				t.Fatal("event channel closed early")
			}
			if ev.Name == "file-loaded" {
				return
			}
		case <-deadline:
			t.Fatal("never saw file-loaded")
		}
	}
}

func TestEventsAndDoneCloseOnClose(t *testing.T) {
	p := newTestPlayer(t)

	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done not closed after Close")
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-p.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("event channel not closed after Close")
		}
	}
}

func TestPlayIndexRejectsOutOfRange(t *testing.T) {
	p := newTestPlayer(t)

	if err := p.PlayIndex(0); err == nil {
		t.Error("expected an error for an empty playlist")
	}

	p.SetPlaylist(tracks(testFile(t, 5)))
	if err := p.PlayIndex(7); err == nil {
		t.Error("expected an error for an out of range index")
	}
}

// TestTwoDefaultPlayersCoexist is the regression test for the shared socket
// path. With a fixed path, the second New unlinked the first's live socket, so
// the first could no longer be reached at all.
func TestTwoDefaultPlayersCoexist(t *testing.T) {
	file := testFile(t, 30)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	first, err := New()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	t.Cleanup(func() { first.Close() })

	second, err := New()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	t.Cleanup(func() { second.Close() })

	// Both must still be independently controllable.
	for i, p := range []*Player{first, second} {
		p.SetPlaylist(tracks(file))
		if err := p.PlayIndex(0); err != nil {
			t.Fatalf("player %d play: %v", i, err)
		}
		wantState(t, p, StatePlaying)
	}

	// Pausing one must not disturb the other.
	if err := first.Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}
	wantState(t, first, StatePaused)
	wantState(t, second, StatePlaying)
}

func TestDurationIsReported(t *testing.T) {
	file := testFile(t, 12)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	p := newTestPlayer(t)
	p.SetPlaylist(tracks(file))
	if err := p.PlayIndex(0); err != nil {
		t.Fatalf("play: %v", err)
	}

	// mpv only learns the duration once the file is decoded.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if p.Duration() == 12*time.Second {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("duration = %s, want 12s", p.Duration())
}

func TestPositionAdvancesAndTracksSeek(t *testing.T) {
	file := testFile(t, 60)
	if file == "" {
		t.Skip("ffmpeg unavailable")
	}

	p := newTestPlayer(t)
	p.SetPlaylist(tracks(file))
	if err := p.PlayIndex(0); err != nil {
		t.Fatalf("play: %v", err)
	}

	waitFor := func(what string, d time.Duration) time.Duration {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if pos := p.Position(); pos >= d {
				return pos
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("position never reached %s (what: %s)", d, what)
		return 0
	}

	waitFor("1s of playback", time.Second)

	// A seek must show up in the position, not just in the audio.
	if err := p.Seek(30 * time.Second); err != nil {
		t.Fatalf("seek: %v", err)
	}
	waitFor("after seeking", 31*time.Second)
}

func TestProgressIsBoundedAndZeroWhenUnknown(t *testing.T) {
	p := newTestPlayer(t)

	if got := p.Progress(); got != 0 {
		t.Errorf("progress with nothing loaded = %v, want 0", got)
	}

	// A position past the end must not produce a fraction above one.
	p.mu.Lock()
	p.pos, p.dur = 30*time.Second, 10*time.Second
	p.mu.Unlock()

	if got := p.Progress(); got != 1 {
		t.Errorf("progress past the end = %v, want 1", got)
	}
}

func TestPositionEventsAreNotForwarded(t *testing.T) {
	// mpv reports position about a dozen times a second. Forwarding it would
	// crowd out the events that actually change state.
	pos := Event{Name: "property-change", Raw: []byte(`{"event":"property-change","name":"time-pos","data":1.5}`)}
	dur := Event{Name: "property-change", Raw: []byte(`{"event":"property-change","name":"duration","data":60}`)}
	pause := Event{Name: "property-change", Raw: []byte(`{"event":"property-change","name":"pause","data":true}`)}
	end := Event{Name: "end-file", Raw: []byte(`{"event":"end-file","reason":"eof"}`)}

	for _, ev := range []Event{pos, dur} {
		if forwardable(ev) {
			t.Errorf("%s should not be forwarded", ev.Raw)
		}
	}
	for _, ev := range []Event{pause, end} {
		if !forwardable(ev) {
			t.Errorf("%s should be forwarded", ev.Raw)
		}
	}
}

func TestMalformedPropertyChangeIsIgnored(t *testing.T) {
	p := newTestPlayer(t)

	// None of these may panic or move the player into a wrong state.
	for _, raw := range []string{
		`not json`,
		`{"event":"property-change"}`,
		`{"event":"property-change","name":"time-pos"}`,
		`{"event":"property-change","name":"pause","data":"yes"}`,
		`{"event":"property-change","name":"duration","data":null}`,
	} {
		// apply takes the lock itself, so it must not be held here.
		p.apply(Event{Name: "property-change", Raw: []byte(raw)})
	}

	if got := p.State(); got != StateStopped {
		t.Errorf("state = %s, want stopped", got)
	}
	if got := p.Duration(); got != 0 {
		t.Errorf("duration = %s, want 0", got)
	}
}
