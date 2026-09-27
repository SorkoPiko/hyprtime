package tracker

import (
	"fmt"
	"strings"
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/db"
	"github.com/ayanrajpoot10/hyprtime/internal/idle"
	"github.com/ayanrajpoot10/hyprtime/internal/logger"
)

type Tracker struct {
	db     *db.DB
	ipc    *IPC
	events chan string
	stop   chan struct{}
	done   chan struct{}

	idleTimeout time.Duration
	idleEvents  <-chan bool
	idleErrs    <-chan error
	idle        bool

	currentApp string
	currentID  int64
	lastUpdate time.Time
	// When the current app's active stretch began (focus or resume from idle)
	segmentStart time.Time
}

// New creates a tracker. An idleTimeout of zero disables idle detection.
func New(db *db.DB, idleTimeout time.Duration) *Tracker {
	return &Tracker{
		db:          db,
		events:      make(chan string, 100),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		idleTimeout: idleTimeout,
	}
}

func (t *Tracker) Start() error {
	logger.Infof("Init IPC")

	var err error
	t.ipc, err = NewIPC()
	if err != nil {
		return fmt.Errorf("init IPC: %w", err)
	}

	logger.Debugf("Subscribing to events")
	if err := t.ipc.Subscribe(t.events); err != nil {
		return fmt.Errorf("subscribe events: %w", err)
	}

	if t.idleTimeout > 0 {
		t.idleEvents, t.idleErrs = idle.Watch(t.idleTimeout)
		logger.Infof("Idle detection enabled (%s)", t.idleTimeout)
	}

	go t.run()

	logger.Infof("Tracking started")

	return nil
}

func (t *Tracker) Stop() {
	logger.Infof("Stopping tracker")

	close(t.stop)
	<-t.done

	logger.Infof("Tracker stopped")
}

func (t *Tracker) run() {
	defer close(t.done)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.stop:
			t.flush()
			return

		case event := <-t.events:
			if event == "" {
				continue
			}
			kind, data := ParseEvent(event)

			if kind == "activewindow" {
				logger.Debugf("Event: %s >> %s", kind, data)
				fields := strings.SplitN(data, ",", 2)
				t.handleFocusChange(fields[0])
			}

		case idle := <-t.idleEvents:
			t.handleIdle(idle)

		case err := <-t.idleErrs:
			logger.Errorf("Idle detection stopped, tracking without it: %v", err)
			t.idleErrs = nil
			t.idleEvents = nil
			t.handleIdle(false)

		case <-ticker.C:
			t.flush()
		}
	}
}

func (t *Tracker) flush() {
	if t.currentApp == "" || t.idle {
		return
	}

	t.record(time.Now())
}

// record adds the time from lastUpdate until the given moment to the current app.
func (t *Tracker) record(until time.Time) {
	duration := int64(until.Sub(t.lastUpdate).Seconds())

	if duration > 0 {
		if err := t.db.AddUsage(t.currentID, duration); err != nil {
			logger.Errorf("Update time: %v", err)
		} else {
			logger.Debugf("Update: %s (%.1fs)", t.currentApp, float64(duration))
			t.lastUpdate = until
		}
	}
}

func (t *Tracker) handleIdle(idle bool) {
	if idle == t.idle {
		return
	}

	now := time.Now()

	if !idle {
		t.idle = false
		t.lastUpdate = now
		t.segmentStart = now
		logger.Infof("Active again, resuming")
		return
	}

	// The notification arrives idleTimeout after the last input, so the user
	// actually left at idleStart. Count up to there and no further.
	if t.currentApp != "" {
		idleStart := now.Add(-t.idleTimeout)

		if t.lastUpdate.Before(idleStart) {
			t.record(idleStart)
		} else {
			// A periodic flush already counted past idleStart; take that back,
			// but only the part that belongs to the current app's stretch
			from := idleStart
			if t.segmentStart.After(from) {
				from = t.segmentStart
			}
			over := int64(t.lastUpdate.Sub(from).Seconds())
			if err := t.db.RemoveUsage(t.currentID, over); err != nil {
				logger.Errorf("Remove idle time: %v", err)
			} else if over > 0 {
				logger.Debugf("Removed %ds idle time from %s", over, t.currentApp)
			}
		}
	}

	t.idle = true
	logger.Infof("Idle for %s, pausing", t.idleTimeout)
}

func (t *Tracker) handleFocusChange(class string) {
	if t.currentApp == class {
		return
	}

	t.flush()

	if class == "" {
		t.currentApp = ""
		return
	}

	appID, err := t.db.AppID(class)
	if err != nil {
		logger.Errorf("Get/create app: %v", err)
		t.currentApp = ""
		return
	}

	t.currentApp = class
	t.currentID = appID
	t.lastUpdate = time.Now()
	t.segmentStart = t.lastUpdate

	logger.Infof("Focus: %s", class)
}
