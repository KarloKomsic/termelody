package ipc

// Play sends a loadfile command to mpv. This replaces any
// currently playing track.
func (m *MPV) Play(filePath string) error {
	_, err := m.Command([]any{"loadfile", filePath})
	return err
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
