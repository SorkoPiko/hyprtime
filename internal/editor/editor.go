// Package editor tracks modal editors (neovim) so that keys pressed outside
// insert mode, which are commands rather than text, aren't counted as typing.
//
// Editors connect to a unix socket and send lines of the form
//
//	<pid> <focused 0|1> <mode>
//
// where mode is neovim's mode string (nvim_get_mode().mode).
package editor

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/ayanrajpoot10/hyprtime/internal/logger"
	"github.com/ayanrajpoot10/hyprtime/internal/rundir"
)

type instance struct {
	focused bool
	mode    string
}

type Tracker struct {
	mu        sync.Mutex
	instances map[int]instance
}

func SocketPath() string {
	return rundir.Path("editor.sock")
}

// Listen starts accepting editor reports on the socket.
func Listen() (*Tracker, error) {
	path := SocketPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", path, err)
	}

	t := &Tracker{instances: map[int]instance{}}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					logger.Errorf("Editor socket: %v", err)
				}
				return
			}
			go t.serve(conn)
		}
	}()
	return t, nil
}

func (t *Tracker) serve(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		t.report(scanner.Text())
	}
}

func (t *Tracker) report(line string) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if fields[2] == "exit" {
		delete(t.instances, pid)
		return
	}
	t.instances[pid] = instance{focused: fields[1] == "1", mode: fields[2]}
	logger.Debugf("Editor %d: focused=%s mode=%s", pid, fields[1], fields[2])
}

// Suppressed reports whether a focused editor is in a mode where keys are
// commands, not text.
func (t *Tracker) Suppressed() bool {
	if t == nil {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for pid, inst := range t.instances {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			// Exited without saying so (crash, kill -9)
			delete(t.instances, pid)
			continue
		}
		if inst.focused && !typingMode(inst.mode) {
			return true
		}
	}
	return false
}

// typingMode is true for insert, replace and terminal modes.
func typingMode(mode string) bool {
	return strings.HasPrefix(mode, "i") ||
		strings.HasPrefix(mode, "R") ||
		mode == "t"
}
