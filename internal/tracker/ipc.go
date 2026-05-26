package tracker

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/ayanrajpoot10/hyprtime/internal/logger"
)

type IPC struct {
	socketPath string
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

	socketPath := fmt.Sprintf("%s/hypr/%s/.socket2.sock", runtimeDir, instance)

	return &IPC{
		socketPath: socketPath,
	}, nil
}

func (i *IPC) Subscribe(events chan<- string) error {
	conn, err := net.Dial("unix", i.socketPath)
	if err != nil {
		return fmt.Errorf("connect event socket: %w", err)
	}

	go func() {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			event, err := reader.ReadString('\n')
			if err != nil {
				logger.Errorf("Read events: %v", err)
				return
			}
			event = strings.TrimSpace(event)
			if event != "" {
				events <- event
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
