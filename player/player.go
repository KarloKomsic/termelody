package player

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/karlokomsic/termelody/ipc"
	"codeberg.org/karlokomsic/termelody/playlist"
)

type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
)

func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StatePlaying:
		return "playing"
	case StatePaused:
		return "paused"
	default:
		return "unknown"
	}
}

// Player manages mpv playback and tracks the current state.
type Player struct {
	mpv    *ipc.MPV
	state  State
	track  playlist.Track
}

// New creates a Player and starts an mpv instance in the background.
func New() (*Player, error) {
	socketPath := filepath.Join(os.TempDir(), "termelody-mpv.sock")

	// Remove stale socket if one exists from a previous run.
	os.Remove(socketPath)

	// Launch mpv with no file — we just want it listening on the
	// socket so we can send commands later.
	mpv, err := ipc.LaunchEmpty(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	return &Player{
		mpv:   mpv,
		state: StateStopped,
	}, nil
}

// Play starts playing a track. If something is already playing,
// it stops that first.
func (p *Player) Play(track playlist.Track) error {
	if err := p.mpv.Play(track.Path); err != nil {
		return fmt.Errorf("failed to play %s: %w", track.Path, err)
	}

	p.state = StatePlaying
	p.track = track
	return nil
}

// Pause pauses playback.
func (p *Player) Pause() error {
	if err := p.mpv.Pause(); err != nil {
		return err
	}
	p.state = StatePaused
	return nil
}

// Resume unpauses playback.
func (p *Player) Resume() error {
	if err := p.mpv.Resume(); err != nil {
		return err
	}
	p.state = StatePlaying
	return nil
}

// TogglePause flips between paused and playing.
func (p *Player) TogglePause() error {
	if err := p.mpv.TogglePause(); err != nil {
		return err
	}

	if p.state == StatePlaying {
		p.state = StatePaused
	} else if p.state == StatePaused {
		p.state = StatePlaying
	}
	return nil
}

// Stop stops playback entirely.
func (p *Player) Stop() error {
	if err := p.mpv.Stop(); err != nil {
		return err
	}
	p.state = StateStopped
	return nil
}

// Close shuts down the mpv process.
func (p *Player) Close() error {
	return p.mpv.Quit()
}

// State returns the current playback state.
func (p *Player) State() State {
	return p.state
}

// Track returns the currently loaded track.
func (p *Player) Track() playlist.Track {
	return p.track
}
