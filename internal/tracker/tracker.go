package tracker

import (
	"fmt"
	"strings"
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/db"
	"github.com/ayanrajpoot10/hyprtime/internal/logger"
)

type Tracker struct {
	db     *db.DB
	ipc    *IPC
	events chan string
	stop   chan struct{}
	done   chan struct{}

	currentApp string
	currentID  int64
	lastUpdate time.Time
}

func New(db *db.DB) *Tracker {
	return &Tracker{
		db:     db,
		events: make(chan string, 100),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
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

		case <-ticker.C:
			t.flush()
		}
	}
}

func (t *Tracker) flush() {
	if t.currentApp == "" {
		return
	}

	now := time.Now()
	duration := int64(now.Sub(t.lastUpdate).Seconds())

	if duration > 0 {
		if err := t.db.AddUsage(t.currentID, duration); err != nil {
			logger.Errorf("Update time: %v", err)
		} else {
			logger.Debugf("Update: %s (%.1fs)", t.currentApp, float64(duration))
			t.lastUpdate = now
		}
	}
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

	logger.Infof("Focus: %s", class)
}
