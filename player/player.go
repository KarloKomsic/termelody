package player

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"codeberg.org/karlokomsic/termelody/ipc"
	"codeberg.org/karlokomsic/termelody/playlist"
)

// Event is a playback event reported by mpv. It is an alias so that callers
// can consume events without importing the ipc package.
type Event = ipc.Event

// stateEventBuffer is how many events may queue for consumers. The forwarder
// blocks once it is full rather than dropping, because losing an event such as
// end-file would silently break auto-advance.
const stateEventBuffer = 32

// maxVolume is the ceiling in percent. 100 is no amplification, so the level
// can be lowered freely but never pushed louder than the source.
const maxVolume = 100

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

// Player manages mpv playback.
//
// Playback state is not tracked by assuming commands succeeded. A single
// goroutine consumes the events mpv pushes and is the only writer of state, so
// what State reports is what mpv is actually doing. That also means state
// settles a moment after a command rather than instantly.
type Player struct {
	mpv    *ipc.MPV
	events chan Event

	// mu guards the fields the event goroutine writes and other goroutines
	// read. tracks and index are untouched by that goroutine, so they stay
	// under the control of whoever drives the player.
	mu     sync.Mutex
	state  State
	loaded bool
	pos    time.Duration
	dur    time.Duration
	volume float64

	tracks []playlist.Track
	index  int
}

// Config holds the optional settings of a Player.
type Config struct {
	// SocketPath is the Unix socket mpv listens on. Leaving it empty gives
	// the Player a private socket of its own, which is what allows several
	// instances to run at the same time without disturbing each other.
	SocketPath string
}

// New creates a Player and starts an mpv instance in the background, using
// default settings.
func New() (*Player, error) {
	return NewWithConfig(Config{})
}

// NewWithConfig creates a Player with the given settings and starts an mpv
// instance in the background.
func NewWithConfig(cfg Config) (*Player, error) {
	// An empty path resolves to a private socket; ipc owns that choice.
	mpv, err := ipc.LaunchEmpty(cfg.SocketPath)
	if err != nil {
		return nil, err
	}

	// mpv pushes no event when the pause property is set, so the only way to
	// know it changed is to ask for it. Observation is per-connection, which
	// ipc handles by writing to the listener.
	//
	// Position is observed too, but only so it can be stored. mpv reports it
	// around a dozen times a second, which is far more often than a display
	// needs to redraw, so it is deliberately not forwarded to consumers.
	//
	// Volume is observed for the same reason as pause: it only arrives when
	// asked for, and the readout has to report what mpv thinks rather than a
	// number this process kept for itself.
	for _, name := range []string{"pause", "time-pos", "duration", "volume"} {
		if err := mpv.ObserveProperty(name); err != nil {
			_ = mpv.Quit()
			return nil, fmt.Errorf("failed to observe %s: %w", name, err)
		}
	}

	// volume-max clamps mpv's add command but not an absolute set of the same
	// property, and volume is only ever changed with add, so this is what
	// keeps it from being amplified past unity.
	if err := mpv.SetProperty("volume-max", maxVolume); err != nil {
		_ = mpv.Quit()
		return nil, fmt.Errorf("failed to set volume-max: %w", err)
	}

	// Reading volume rather than starting at mpv's default of 100 keeps the
	// first report honest. Observing sends the current value too, but only
	// once the event goroutine below drains it, so a render that beat it
	// would otherwise claim silence.
	raw, err := mpv.GetProperty("volume")
	if err != nil {
		_ = mpv.Quit()
		return nil, fmt.Errorf("failed to read volume: %w", err)
	}
	volume, ok := raw.(float64)
	if !ok {
		_ = mpv.Quit()
		return nil, fmt.Errorf("volume came back as %T, want a number", raw)
	}

	p := &Player{
		mpv:    mpv,
		events: make(chan Event, stateEventBuffer),
		state:  StateStopped,
		volume: volume,
	}

	go p.watch()

	return p, nil
}

// watch converts mpv events into player state and forwards them for consumers.
func (p *Player) watch() {
	defer close(p.events)

	for ev := range p.mpv.Events() {
		p.apply(ev)

		if !forwardable(ev) {
			continue
		}

		select {
		case p.events <- ev:
		case <-p.mpv.Done():
			// mpv has gone away, so there is nothing left worth forwarding.
			return
		}
	}
}

// forwardable reports whether an event is worth passing on. Position is left
// out on purpose: mpv emits it far faster than any display redraws, and
// forwarding it would fill the channel and stall the event loop behind it.
func forwardable(ev Event) bool {
	if ev.Name != "property-change" {
		return true
	}
	switch property(ev) {
	case "time-pos", "duration":
		return false
	default:
		return true
	}
}

// property returns the name of a property-change event, or "" for any other.
func property(ev Event) string {
	var change struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(ev.Raw, &change); err != nil {
		return ""
	}
	return change.Name
}

// EndedNaturally reports whether an end-file event says playback reached the
// end of the file on its own. Checking the reason rather than the event name
// is what makes this safe: loadfile and stop both emit end-file for the file
// they interrupt, and counting those as a finished track would skip ahead
// while the caller is still choosing what plays next. A payload that will not
// parse counts as "no", because a broken event must not move the playlist.
func EndedNaturally(ev Event) bool {
	if ev.Name != "end-file" {
		return false
	}

	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(ev.Raw, &payload); err != nil {
		return false
	}
	return payload.Reason == "eof"
}

// apply folds a single event into player state.
func (p *Player) apply(ev Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch ev.Name {
	case "file-loaded":
		p.state = StatePlaying
		p.loaded = true
		p.pos = 0

	case "end-file", "idle":
		// Playback of the current file finished or was stopped. Note that
		// loadfile also produces end-file for the file being replaced, and
		// that arrives before the replacement's file-loaded, so the final
		// state still settles on playing.
		p.state = StateStopped
		p.loaded = false
		p.pos = 0
		p.dur = 0

	case "property-change":
		p.applyProperty(ev.Raw)
	}
}

// applyProperty folds a single property change into player state. Only the
// properties the player cares about are looked at.
func (p *Player) applyProperty(raw []byte) {
	// data is left raw because its type depends on the property: pause is a
	// boolean while time-pos and duration are numbers. Decoding it as one
	// fixed type would make every other property fail to parse.
	//
	// mpv also omits data entirely when a property is unavailable, so an
	// empty value means "unknown" rather than a real zero.
	var change struct {
		Name string          `json:"name"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &change); err != nil || len(change.Data) == 0 {
		return
	}

	switch change.Name {
	case "pause":
		// Ignore pause changes while stopped: a pause property is untouched
		// by stop, so unpausing later must not invent a playing state.
		var paused bool
		if json.Unmarshal(change.Data, &paused) != nil || !p.loaded {
			return
		}
		if paused {
			p.state = StatePaused
		} else {
			p.state = StatePlaying
		}

	case "time-pos":
		p.pos = p.seconds(change.Data)

	case "duration":
		p.dur = p.seconds(change.Data)

	case "volume":
		// seconds must not be used here: it drops zero as "unknown", but zero
		// volume is silence, a perfectly real reading.
		var v float64
		if json.Unmarshal(change.Data, &v) == nil {
			p.volume = v
		}
	}
}

// seconds decodes an mpv value in seconds, treating anything unusable as
// unknown rather than as a position.
func (p *Player) seconds(data json.RawMessage) time.Duration {
	var v float64
	if err := json.Unmarshal(data, &v); err != nil || v <= 0 {
		return 0
	}
	return time.Duration(v * float64(time.Second))
}

// Play starts playing a track, replacing whatever is loaded.
func (p *Player) Play(track playlist.Track) error {
	if err := p.mpv.Play(track.Path); err != nil {
		return fmt.Errorf("failed to play %s: %w", track.Path, err)
	}
	return nil
}

// PlayIndex plays the track at the given position in the playlist.
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

// Pause pauses playback.
func (p *Player) Pause() error {
	return p.mpv.Pause()
}

// Resume unpauses playback.
func (p *Player) Resume() error {
	return p.mpv.Resume()
}

// TogglePause flips between playing and paused. It deliberately does not
// decide which way to flip based on State: mpv reports the outcome, so this
// cannot disagree with reality.
func (p *Player) TogglePause() error {
	return p.mpv.TogglePause()
}

// Stop stops playback entirely.
func (p *Player) Stop() error {
	return p.mpv.Stop()
}

// AdjustVolume moves the volume by delta percent, where negative lowers it.
// The change is made inside mpv with add rather than computed here from the
// last observed value, so a run of quick presses cannot lose a step to a copy
// that has not caught up yet. volume-max, set when the Player starts, clamps
// add but not an absolute set, so the level cannot pass maxVolume either.
func (p *Player) AdjustVolume(delta int) error {
	if err := p.mpv.AddToProperty("volume", float64(delta)); err != nil {
		return fmt.Errorf("failed to adjust volume: %w", err)
	}
	return nil
}

// Seek shifts playback by delta, where a negative delta seeks backwards.
// Seeking with nothing loaded is a no-op, since mpv has no position to move.
func (p *Player) Seek(delta time.Duration) error {
	p.mu.Lock()
	loaded := p.loaded
	p.mu.Unlock()

	if !loaded {
		return nil
	}

	return p.mpv.SeekRelative(delta.Seconds())
}

// Events returns the stream of playback events. It is closed when mpv exits.
func (p *Player) Events() <-chan Event {
	return p.events
}

// Done is closed once the mpv process has exited, whether from Quit or a crash.
func (p *Player) Done() <-chan struct{} {
	return p.mpv.Done()
}

// State returns what mpv is currently doing.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Position returns how far into the track playback has reached. It is zero
// when nothing is loaded, and it does not advance while paused, because mpv
// stops reporting a moving position.
func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pos
}

// Duration returns the length of the loaded track, or zero when mpv does not
// know it, which is the case for streams and for some containers.
func (p *Player) Duration() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dur
}

// Volume returns the current volume in percent as mpv reports it, rounded
// since the level is presented as a whole number. It is read from the value
// mpv pushed rather than counted locally, so it cannot drift from reality.
func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return int(math.Round(p.volume))
}

// Progress returns how much of the track has been played, from 0 to 1. It is
// zero when the duration is unknown, so a caller can treat it as a plain
// fraction without special-casing.
func (p *Player) Progress() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.dur <= 0 {
		return 0
	}
	return min(float64(p.pos)/float64(p.dur), 1)
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

// Close shuts down the mpv process and waits for it to exit.
func (p *Player) Close() error {
	return p.mpv.Quit()
}
