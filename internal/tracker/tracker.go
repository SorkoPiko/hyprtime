package tracker

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/db"
	"github.com/ayanrajpoot10/hyprtime/internal/editor"
	"github.com/ayanrajpoot10/hyprtime/internal/idle"
	"github.com/ayanrajpoot10/hyprtime/internal/keys"
	"github.com/ayanrajpoot10/hyprtime/internal/logger"
	"github.com/ayanrajpoot10/hyprtime/internal/typing"
)

type Options struct {
	// Pause after this long without input; zero disables idle detection
	IdleTimeout time.Duration
	// Count typing statistics
	Typing bool
	// Window classes whose keys aren't counted as typing (games)
	TypingExclude *regexp.Regexp
}

type Tracker struct {
	db     *db.DB
	ipc    *IPC
	events chan string
	stop   chan struct{}
	done   chan struct{}

	opts        Options
	idleTimeout time.Duration
	idleEvents  <-chan bool
	idleErrs    <-chan error
	idle        bool

	currentApp string
	currentID  int64
	lastUpdate time.Time
	// When the current app's active stretch began (focus or resume from idle)
	segmentStart time.Time

	keys      *keys.Watcher
	keyEvents <-chan typing.Key
	counter   *typing.Counter
	editors   *editor.Tracker
	lastState time.Time
}

func New(db *db.DB, opts Options) *Tracker {
	return &Tracker{
		db:          db,
		events:      make(chan string, 100),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		opts:        opts,
		idleTimeout: opts.IdleTimeout,
		counter:     typing.NewCounter(),
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

	if t.opts.Typing {
		if err := t.startTyping(); err != nil {
			logger.Errorf("Typing stats disabled: %v", err)
		}
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

	stateTicker := time.NewTicker(1 * time.Second)
	defer stateTicker.Stop()

	for {
		select {
		case <-t.stop:
			t.flush()
			t.counter.Finish()
			t.flushTyping()
			return

		case event := <-t.events:
			if event == "" {
				continue
			}
			kind, data := ParseEvent(event)

			switch kind {
			case "activewindow":
				logger.Debugf("Event: %s >> %s", kind, data)
				fields := strings.SplitN(data, ",", 2)
				t.handleFocusChange(fields[0])
			case "activelayout":
				t.reloadLayout()
			}

		case k := <-t.keyEvents:
			t.handleKey(k)

		case now := <-stateTicker.C:
			// Publish while typing (live speed) and once when a burst ends
			if t.counter.Tick(now) || now.Sub(t.lastState) < 3*time.Second {
				t.writeState(now)
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
			t.flushTyping()
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
	t.counter.ResetBuffer()

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

func (t *Tracker) startTyping() error {
	layout, err := t.ipc.KeyboardLayout()
	if err != nil {
		return fmt.Errorf("keyboard layout: %w", err)
	}

	w, err := keys.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.SetLayout(layout); err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	totals, err := t.db.TypingByDate(today)
	if err != nil {
		return fmt.Errorf("load today's typing: %w", err)
	}
	bursts, err := t.db.BurstsByDate(today)
	if err != nil {
		return fmt.Errorf("load today's bursts: %w", err)
	}
	t.counter.Restore(today, totals, bursts)

	t.editors, err = editor.Listen()
	if err != nil {
		logger.Errorf("Editor mode tracking disabled: %v", err)
	}

	w.Start()
	t.keys = w
	t.keyEvents = w.Keys()
	t.writeState(time.Now())

	logger.Infof("Typing stats enabled")
	return nil
}

func (t *Tracker) reloadLayout() {
	if t.keys == nil {
		return
	}
	layout, err := t.ipc.KeyboardLayout()
	if err != nil {
		logger.Errorf("Keyboard layout: %v", err)
		return
	}
	if err := t.keys.SetLayout(layout); err != nil {
		logger.Errorf("Set keyboard layout: %v", err)
	}
}

func (t *Tracker) handleKey(k typing.Key) {
	if t.currentApp == "" ||
		(t.opts.TypingExclude != nil && t.opts.TypingExclude.MatchString(t.currentApp)) ||
		t.editors.Suppressed() {
		t.counter.ResetBuffer()
		return
	}

	t.counter.Handle(k, t.currentID)

	if k.Time.Sub(t.lastState) >= 500*time.Millisecond {
		t.writeState(k.Time)
	}
}

func (t *Tracker) writeState(now time.Time) {
	if t.keys == nil {
		return
	}
	t.lastState = now
	if err := typing.WriteState(t.counter.Snapshot(now)); err != nil {
		logger.Errorf("Write typing state: %v", err)
	}
}

func (t *Tracker) flushTyping() {
	totals, bursts := t.counter.Drain()

	for app, tot := range totals {
		if err := t.db.AddTyping(app, tot); err != nil {
			logger.Errorf("Save typing: %v", err)
		}
	}
	for _, b := range bursts {
		if err := t.db.AddBurst(b); err != nil {
			logger.Errorf("Save burst: %v", err)
		}
	}
}
