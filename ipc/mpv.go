package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const (
	// eventBuffer is how many unread events may queue. The reader blocks
	// once it is full, applying backpressure rather than dropping events:
	// losing an "end-file" would mean never auto-advancing.
	eventBuffer = 64

	// startupTimeout bounds how long we wait for mpv to create its socket.
	startupTimeout = 5 * time.Second
)

// Event is a notification pushed by mpv, as opposed to a reply to a command.
type Event struct {
	// Name is the event name, such as "end-file", "pause" or "idle".
	Name string

	// Raw is the entire JSON object as mpv sent it. Event payloads are
	// flattened alongside "event" rather than nested under "data", and
	// every event carries a different set of fields, so the whole object is
	// passed through for callers to decode the parts they care about.
	Raw json.RawMessage
}

// message is the union of a command reply and an event. mpv multiplexes both
// over the same connection, so every line has to be classified before it can
// be used. The two shapes differ: a reply is
// {"error":"success","data":...} while an event is
// {"event":"end-file","reason":"eof",...}.
type message struct {
	Event string          `json:"event"`
	Error string          `json:"error"`
	Data  json.RawMessage `json:"data"`
}

func (m message) isEvent() bool { return m.Event != "" }

type commandRequest struct {
	Command []any `json:"command"`
}

// MPV is a client for an mpv instance, and owns the process when it started it.
type MPV struct {
	socketPath string
	socketDir  string

	// cmd is non-nil only when this client launched mpv itself.
	cmd *exec.Cmd

	// events carries notifications pushed by mpv. It is closed when mpv
	// exits, so a range over it terminates on its own.
	events chan Event

	// done is closed once the mpv process has been reaped.
	done chan struct{}

	// waitErr is written by reap and must only be read after done is closed,
	// which is what provides the happens-before edge.
	waitErr error

	listenOnce sync.Once

	// listener is the long-lived connection that receives pushed events,
	// guarded by writeMu for the rare case of a write to it.
	listener net.Conn
	writeMu  sync.Mutex
	nextObs  int
}

// NewMPV returns a client for an mpv instance that somebody else owns. It only
// supports command requests; Events returns nil because subscribing requires a
// listener connection that this constructor does not open. Use Launch or
// LaunchEmpty when termelody manages the process itself.
func NewMPV(socketPath string) *MPV {
	return &MPV{socketPath: socketPath}
}

// Launch starts mpv playing filePath and returns a client that also subscribes
// to its events. An empty socketPath means a private socket is chosen
// automatically; see LaunchEmpty.
func Launch(socketPath string, filePath string) (*MPV, error) {
	return start(socketPath, filePath, false)
}

// LaunchEmpty starts mpv with no file and returns a client that also subscribes
// to its events. The --idle flag is what keeps mpv alive: without a file it
// would otherwise exit immediately.
//
// An empty socketPath gives this instance its own private socket under the
// user's runtime directory, so two copies of termelody can run side by side. A
// fixed shared path cannot work: a second instance clears what it assumes is a
// stale socket, which would silently unlink a live one belonging to the first.
func LaunchEmpty(socketPath string) (*MPV, error) {
	return start(socketPath, "", true)
}

// resolveSocketPath decides where mpv should listen. A caller-supplied path is
// honoured verbatim and no directory is created, which is what lets tests and
// deliberate sharing keep working. Otherwise each instance gets a private 0700
// directory, so its socket cannot be reached or removed by anyone else.
func resolveSocketPath(requested string) (path string, dir string, err error) {
	if requested != "" {
		return requested, "", nil
	}

	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		// Unlike XDG_RUNTIME_DIR, the temp directory is world-writable, so
		// the socket goes inside a private directory rather than into it.
		base = os.TempDir()
	}

	dir, err = os.MkdirTemp(base, "termelody-")
	if err != nil {
		return "", "", fmt.Errorf("could not create runtime directory: %w", err)
	}

	return filepath.Join(dir, "mpv.sock"), dir, nil
}

// start launches mpv, waits for it to create its socket, and begins listening
// for events. On any failure the process is torn down so no orphan is left
// behind, since nothing would be left to quit it.
func start(requested string, filePath string, idle bool) (*MPV, error) {
	socketPath, socketDir, err := resolveSocketPath(requested)
	if err != nil {
		return nil, err
	}

	// A socket left behind by a previous run would make mpv fail to bind. The
	// path is private to this instance, so anything here really is stale.
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanupSocketDir(socketDir)
		return nil, fmt.Errorf("could not clear stale socket: %w", err)
	}

	args := []string{"--input-ipc-server=" + socketPath}
	if filePath != "" {
		args = append(args, filePath)
	}
	if idle {
		args = append(args, "--idle")
	}

	cmd := exec.Command("mpv", append([]string{"--no-video", "--no-terminal"}, args...)...)
	if err := cmd.Start(); err != nil {
		// No process exists, so nothing will reap the directory for us.
		cleanupSocketDir(socketDir)
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	m := &MPV{
		socketPath: socketPath,
		socketDir:  socketDir,
		cmd:        cmd,
		events:     make(chan Event, eventBuffer),
		done:       make(chan struct{}),
	}

	go m.reap()

	if err := waitForSocket(socketPath, startupTimeout); err != nil {
		m.terminate()
		return nil, err
	}

	if err := m.listen(); err != nil {
		m.terminate()
		return nil, err
	}

	return m, nil
}

// cleanupSocketDir removes a private socket directory, ignoring the case where
// there is not one because the caller supplied the socket path.
func cleanupSocketDir(dir string) {
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

// reap waits for the mpv process so that it is not left as a zombie, and
// removes the socket file once it is gone. Running in its own goroutine is
// what lets the UI notice that mpv died instead of blocking on it.
func (m *MPV) reap() {
	m.waitErr = m.cmd.Wait()
	_ = os.Remove(m.socketPath)
	cleanupSocketDir(m.socketDir)
	close(m.done)
}

// listen opens the dedicated connection that receives pushed events.
func (m *MPV) listen() error {
	var err error
	m.listenOnce.Do(func() {
		var conn net.Conn
		if conn, err = net.Dial("unix", m.socketPath); err != nil {
			err = fmt.Errorf("could not subscribe to mpv events: %w", err)
			return
		}

		m.listener = conn
		go m.readEvents(conn)
	})
	return err
}

// ObserveProperty asks mpv to push a property-change event whenever name
// changes. Observation is scoped to the connection that requests it, so this
// writes to the event listener rather than to a throwaway command connection.
//
// This is how playback state is learned reliably: mpv pushes nothing when the
// pause property is set, so the only way to be told about it is to ask.
func (m *MPV) ObserveProperty(name string) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	if m.listener == nil {
		return errors.New("cannot observe properties without an event listener")
	}

	m.nextObs++

	data, err := json.Marshal(commandRequest{
		Command: []any{"observe_property", m.nextObs, name},
	})
	if err != nil {
		return err
	}

	if _, err := m.listener.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("could not observe %s: %w", name, err)
	}

	return nil
}

func (m *MPV) readEvents(conn net.Conn) {
	defer close(m.events)
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// mpv exited or the connection broke; nothing more will arrive.
			return
		}

		var msg message
		if err := json.Unmarshal([]byte(line), &msg); err != nil || !msg.isEvent() {
			continue
		}

		select {
		case m.events <- Event{Name: msg.Event, Raw: json.RawMessage(line)}:
		case <-m.done:
			return
		}
	}
}

// Events returns the channel of events pushed by mpv. It is closed when mpv
// exits. Returns nil for clients built with NewMPV, which do not subscribe.
func (m *MPV) Events() <-chan Event {
	return m.events
}

// Done is closed once the mpv process has exited.
func (m *MPV) Done() <-chan struct{} {
	return m.done
}

// Err reports why mpv exited. It must only be read after Done is closed, since
// that is what guarantees the value is visible.
func (m *MPV) Err() error {
	<-m.done
	return m.waitErr
}

// Command sends a command to mpv and returns its reply data. Events that arrive
// on this connection first are skipped, because mpv pushes them to every
// client, not just the one that asked a question.
func (m *MPV) Command(command []any) (any, error) {
	conn, err := net.Dial("unix", m.socketPath)
	if err != nil {
		return nil, fmt.Errorf("could not connect to mpv: %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(commandRequest{Command: command})
	if err != nil {
		return nil, err
	}

	if _, err := conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		var msg message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.isEvent() {
			continue
		}

		if msg.Error != "success" {
			return nil, fmt.Errorf("mpv command failed: %s", msg.Error)
		}

		// Decode the reply payload so callers get ordinary Go values such as
		// float64 or bool rather than raw JSON. Replies always carry "data",
		// but a few commands have none, which decodes to nil.
		var value any
		if len(msg.Data) > 0 {
			if err := json.Unmarshal(msg.Data, &value); err != nil {
				return nil, err
			}
		}

		return value, nil
	}
}

// Quit asks mpv to exit and waits for the process to be reaped.
func (m *MPV) Quit() error {
	if _, err := m.Command([]any{"quit"}); err != nil {
		// mpv may already be gone, in which case shutting down is moot.
		// Fall through to terminate so the caller still ends up with no
		// running process and no leftover socket.
		m.terminate()
		return err
	}

	<-m.done
	return nil
}

// terminate kills mpv if it is still running and waits for the reaper. It is
// safe to call more than once, which lets both Quit and the startup failure
// paths clean up after themselves.
func (m *MPV) terminate() {
	select {
	case <-m.done:
		return
	default:
	}

	if m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}

	<-m.done
}

// waitForSocket polls until the socket file appears on disk.
func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("mpv socket %s did not appear within %s", path, timeout)
}
