package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

// MPV holds a connection path to an mpv instance's IPC socket.
type MPV struct {
	socketPath string
}

type commandRequest struct {
	Command []any `json:"command"`
}

type commandResponse struct {
	Error string `json:"error"`
	Data  any    `json:"data"`
}

// NewMPV creates a client that can talk to an already-running
// mpv instance listening on socketPath.
func NewMPV(socketPath string) *MPV {
	return &MPV{
		socketPath: socketPath,
	}
}

// Launch starts a new mpv process with the given file, exposing
// its IPC socket at socketPath. It returns an MPV client ready
// to send commands to that process.
func Launch(socketPath string, filePath string) (*MPV, error) {
	cmd := exec.Command(
		"mpv",
		"--no-video",
		"--no-terminal",
		"--input-ipc-server="+socketPath,
		filePath,
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	return &MPV{socketPath: socketPath}, nil
}

// LaunchEmpty starts mpv with no file, just listening on the
// IPC socket. Useful when you want to load files later via
// the loadfile command.
func LaunchEmpty(socketPath string) (*MPV, error) {
	cmd := exec.Command(
		"mpv",
		"--no-video",
		"--no-terminal",
		"--idle",
		"--input-ipc-server="+socketPath,
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	// Wait for the socket to appear before returning.
	if err := waitForSocket(socketPath, 5*time.Second); err != nil {
		return nil, err
	}

	return &MPV{socketPath: socketPath}, nil
}

// Command sends a raw IPC command to mpv and returns the response
// data. For example, to query the current volume:
//
//	data, err := mpv.Command([]any{"get_property", "volume"})
func (m *MPV) Command(command []any) (any, error) {
	conn, err := net.Dial("unix", m.socketPath)
	if err != nil {
		return nil, fmt.Errorf("could not connect to mpv: %w", err)
	}
	defer conn.Close()

	request := commandRequest{
		Command: command,
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	data = append(data, '\n')

	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	var response commandResponse
	if err := json.Unmarshal([]byte(line), &response); err != nil {
		return nil, err
	}

	if response.Error != "success" {
		return nil, fmt.Errorf("mpv command failed: %s", response.Error)
	}

	return response.Data, nil
}

// waitForSocket polls until the socket file appears on disk.
func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := os.Stat(path)
		if err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("mpv socket %s did not appear within %s", path, timeout)
}
