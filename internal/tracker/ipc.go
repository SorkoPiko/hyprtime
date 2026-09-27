package tracker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/ayanrajpoot10/hyprtime/internal/keys"
	"github.com/ayanrajpoot10/hyprtime/internal/logger"
)

type IPC struct {
	socketPath  string
	requestPath string
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
	requestPath := fmt.Sprintf("%s/hypr/%s/.socket.sock", runtimeDir, instance)

	return &IPC{
		socketPath:  socketPath,
		requestPath: requestPath,
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

// Request sends a command to Hyprland's request socket and returns the reply.
func (i *IPC) Request(cmd string) ([]byte, error) {
	conn, err := net.Dial("unix", i.requestPath)
	if err != nil {
		return nil, fmt.Errorf("connect request socket: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(cmd)); err != nil {
		return nil, err
	}
	return io.ReadAll(conn)
}

// KeyboardLayout returns the keymap of Hyprland's main keyboard.
func (i *IPC) KeyboardLayout() (keys.Layout, error) {
	reply, err := i.Request("j/devices")
	if err != nil {
		return keys.Layout{}, err
	}

	var devices struct {
		Keyboards []struct {
			Rules    string `json:"rules"`
			Model    string `json:"model"`
			Layout   string `json:"layout"`
			Variant  string `json:"variant"`
			Options  string `json:"options"`
			NumLock  bool   `json:"numLock"`
			CapsLock bool   `json:"capsLock"`
			Main     bool   `json:"main"`
		} `json:"keyboards"`
	}
	if err := json.Unmarshal(reply, &devices); err != nil {
		return keys.Layout{}, fmt.Errorf("parse devices: %w", err)
	}

	for _, k := range devices.Keyboards {
		if k.Main {
			return keys.Layout{
				Rules: k.Rules, Model: k.Model, Layout: k.Layout,
				Variant: k.Variant, Options: k.Options,
				NumLock: k.NumLock, CapsLock: k.CapsLock,
			}, nil
		}
	}
	return keys.Layout{}, fmt.Errorf("no main keyboard reported")
}
