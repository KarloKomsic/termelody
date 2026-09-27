package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

type MPV struct {
	socketPath string
}

type commandRequest struct {
	Command []any `json:"command"`
}

type commandResponse struct {
	Error string `json:"error"`
}

// We need to start a new
// mpv instance to the termelody socket
func newMPV(socketPath string) *MPV {
	return &MPV{
		socketPath: socketPath,
	}
}

func (m *MPV) Command(command []any) error {
	// First we connect to the socket
	conn, err := net.Dial("unix", m.socketPath)
	if err != nil {
		fmt.Println("Could not connect to mpv:", err)
		return err
	}
	defer conn.Close()

	// Then we handle request, construct
	// a Go value representing the command,
	// and then encode it to JSON
	request := commandRequest{
		Command: command,
	}

	data, err := json.Marshal(request)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	_, err = conn.Write(data)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(conn)

	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// Decoding the command response
	var response commandResponse

	err = json.Unmarshal([]byte(line), &response)
	if err != nil {
		return err
	}

	if response.Error != "success" {
		return fmt.Errorf("mpv command failed: %s", response.Error)
	}

	// We return nil because of Go's
	// error checking
	return err
}
