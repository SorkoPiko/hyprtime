// Package idle reports user idle state via the ext-idle-notify-v1 Wayland protocol.
package idle

//go:generate wayland-scanner client-header /usr/share/wayland-protocols/staging/ext-idle-notify/ext-idle-notify-v1.xml ext-idle-notify-v1-client-protocol.h
//go:generate wayland-scanner private-code /usr/share/wayland-protocols/staging/ext-idle-notify/ext-idle-notify-v1.xml ext-idle-notify-v1-protocol.c

/*
#cgo pkg-config: wayland-client
#include "idle.h"
*/
import "C"

import (
	"fmt"
	"time"
)

var events chan bool

//export goIdleEvent
func goIdleEvent(idle C.int) {
	events <- idle != 0
}

// Watch starts listening for idle/resume transitions after timeout of no input.
// It sends true when the user goes idle and false when they resume. The
// returned error channel receives a value if the monitor stops.
func Watch(timeout time.Duration) (<-chan bool, <-chan error) {
	events = make(chan bool, 8)
	errs := make(chan error, 1)

	go func() {
		switch C.idle_run(C.uint32_t(timeout.Milliseconds())) {
		case C.IDLE_ERR_CONNECT:
			errs <- fmt.Errorf("connect to wayland display")
		case C.IDLE_ERR_UNSUPPORTED:
			errs <- fmt.Errorf("compositor does not support ext-idle-notify-v1")
		default:
			errs <- fmt.Errorf("wayland connection closed")
		}
	}()

	return events, errs
}
