package ipc

import "fmt"

// Play sends a loadfile command to mpv. This replaces any currently playing
// track. Pause is cleared explicitly instead of through mpv's
// --reset-on-next-file=pause, because that reset also undoes the pause of a
// recovered track; without it, a new choice starts playing even if the
// previous track was paused.
func (m *MPV) Play(filePath string) error {
	if _, err := m.Command([]any{"loadfile", filePath}); err != nil {
		return err
	}
	return m.Resume()
}

// PlayAt loads a file and starts it at the given position in seconds,
// optionally paused. Start and pause travel as loadfile options so playback
// never begins at the wrong place, and a paused resume does not make a sound
// first. The pause is set again afterwards because mpv may accept the option
// before the file is ready.
func (m *MPV) PlayAt(filePath string, start float64, paused bool) error {
	options := fmt.Sprintf("start=%.3f", start)
	if paused {
		options += ",pause=yes"
	}

	if _, err := m.Command([]any{"loadfile", filePath, "replace", -1, options}); err != nil {
		return err
	}

	if paused {
		return m.Pause()
	}
	return m.Resume()
}

// Pause pauses playback.
func (m *MPV) Pause() error {
	_, err := m.Command([]any{"set_property", "pause", true})
	return err
}

// Resume unpauses playback.
func (m *MPV) Resume() error {
	_, err := m.Command([]any{"set_property", "pause", false})
	return err
}

// Stop stops playback entirely.
func (m *MPV) Stop() error {
	_, err := m.Command([]any{"stop"})
	return err
}

// SetProperty sets an arbitrary mpv property by name.
func (m *MPV) SetProperty(name string, value any) error {
	_, err := m.Command([]any{"set_property", name, value})
	return err
}

// GetProperty queries an arbitrary mpv property by name.
func (m *MPV) GetProperty(name string) (any, error) {
	return m.Command([]any{"get_property", name})
}

// AddToProperty adds delta to an mpv property. The arithmetic happens inside
// mpv rather than against a value this process has cached, so a run of quick
// presses cannot lose a step to a copy that has not caught up yet.
func (m *MPV) AddToProperty(name string, delta float64) error {
	_, err := m.Command([]any{"add", name, delta})
	return err
}

// TogglePause flips the pause state.
func (m *MPV) TogglePause() error {
	_, err := m.Command([]any{"cycle", "pause"})
	return err
}

// Seek moves to an absolute position in seconds.
func (m *MPV) Seek(seconds float64) error {
	_, err := m.Command([]any{"seek", seconds, "absolute"})
	return err
}

// SeekRelative moves playback by the given number of seconds, where a negative
// amount seeks backwards.
func (m *MPV) SeekRelative(seconds float64) error {
	_, err := m.Command([]any{"seek", seconds, "relative"})
	return err
}
