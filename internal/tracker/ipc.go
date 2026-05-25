package tracker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/ayanrajpoot10/hyprtime/internal/logger"
)

type IPC struct {
	socketPath string
	eventPath  string
}

type ActiveWindow struct {
	Address      string `json:"address"`
	Class        string `json:"class"`
	Title        string `json:"title"`
	InitialClass string `json:"initialClass"`
	InitialTitle string `json:"initialTitle"`
}

func NewIPC() (*IPC, error) {
	instance := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if instance == "" {
		return nil, fmt.Errorf("HYPRLAND_INSTANCE_SIGNATURE not set. Are you running Hyprland?")
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "/run/user/" + os.Getenv("UID")
	}

	socketPath := fmt.Sprintf("%s/hypr/%s/.socket.sock", runtimeDir, instance)
	eventPath := fmt.Sprintf("%s/hypr/%s/.socket2.sock", runtimeDir, instance)

	return &IPC{
		socketPath: socketPath,
		eventPath:  eventPath,
	}, nil
}

func (i *IPC) GetActiveWindow() (*ActiveWindow, error) {
	conn, err := net.Dial("unix", i.socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect socket: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("j/activewindow"))
	if err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}

	buf := make([]byte, 8192)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var window ActiveWindow
	if err := json.Unmarshal(buf[:n], &window); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &window, nil
}

func (i *IPC) SubscribeToEvents(eventChan chan<- string) error {
	conn, err := net.Dial("unix", i.eventPath)
	if err != nil {
		return fmt.Errorf("connect event socket: %w", err)
	}

	go func() {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				logger.Errorf("Read events: %v", err)
				return
			}
			line = strings.TrimSpace(line)
			if line != "" {
				eventChan <- line
			}
		}
	}()

	return nil
}

func ParseEvent(event string) (string, string) {
	parts := strings.SplitN(event, ">>", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
