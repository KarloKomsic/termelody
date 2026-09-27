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
	tracks []playlist.Track
	index  int
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
	return nil
}

// PlayIndex plays the track at the given index in the playlist.
func (p *Player) PlayIndex(i int) error {
	if i < 0 || i >= len(p.tracks) {
		return fmt.Errorf("index %d out of range", i)
	}
	p.index = i
	return p.Play(p.tracks[i])
}

// SetPlaylist sets the track list and resets the index to 0.
func (p *Player) SetPlaylist(tracks []playlist.Track) {
	p.tracks = tracks
	p.index = 0
}

// Next advances to the next track and plays it.
func (p *Player) Next() error {
	if len(p.tracks) == 0 {
		return nil
	}
	next := p.index + 1
	if next >= len(p.tracks) {
		next = 0
	}
	return p.PlayIndex(next)
}

// Prev goes back to the previous track and plays it.
func (p *Player) Prev() error {
	if len(p.tracks) == 0 {
		return nil
	}
	prev := p.index - 1
	if prev < 0 {
		prev = len(p.tracks) - 1
	}
	return p.PlayIndex(prev)
}

// Index returns the current track index in the playlist.
func (p *Player) Index() int {
	return p.index
}

// Track returns the currently loaded track.
func (p *Player) Track() playlist.Track {
	if len(p.tracks) == 0 {
		return playlist.Track{}
	}
	return p.tracks[p.index]
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
