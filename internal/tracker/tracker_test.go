package tracker

import (
	"testing"
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/db"
)

func newTestTracker(t *testing.T, idleTimeout time.Duration) *Tracker {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	d, err := db.New()
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	return New(d, idleTimeout)
}

func focus(t *testing.T, tr *Tracker, class string, at time.Time) {
	t.Helper()
	tr.handleFocusChange(class)
	tr.lastUpdate = at
	tr.segmentStart = at
}

func todayTotal(t *testing.T, tr *Tracker) int64 {
	t.Helper()
	total, err := tr.db.TotalDurationByDate(time.Now().Format("2006-01-02"))
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	return total
}

func TestIdleCountsUpToLastInput(t *testing.T) {
	tr := newTestTracker(t, 2*time.Minute)

	// Focused 5 minutes ago, no flush since; idle fires now, so the user
	// left 2 minutes ago and only 3 minutes should count
	focus(t, tr, "kitty", time.Now().Add(-5*time.Minute))
	tr.handleIdle(true)

	if got := todayTotal(t, tr); got < 179 || got > 181 {
		t.Fatalf("want ~180s, got %ds", got)
	}
}

func TestIdleRemovesOvercountedFlush(t *testing.T) {
	tr := newTestTracker(t, 2*time.Minute)

	// Focused 3 minutes ago and a periodic flush just counted all of it;
	// the last 2 minutes were idle and must be taken back
	focus(t, tr, "kitty", time.Now().Add(-3*time.Minute))
	tr.flush()
	tr.handleIdle(true)

	if got := todayTotal(t, tr); got < 59 || got > 61 {
		t.Fatalf("want ~60s, got %ds", got)
	}
}

func TestIdleOnlyRemovesCurrentStretch(t *testing.T) {
	tr := newTestTracker(t, 2*time.Minute)

	// Focus moved to firefox 30s ago (inside the idle window) and was flushed;
	// only those 30s can be taken back from firefox
	focus(t, tr, "firefox", time.Now().Add(-30*time.Second))
	if err := tr.db.AddUsage(tr.currentID, 600); err != nil {
		t.Fatal(err)
	}
	tr.flush()
	tr.handleIdle(true)

	if got := todayTotal(t, tr); got < 599 || got > 601 {
		t.Fatalf("want ~600s, got %ds", got)
	}
}

func TestNothingCountedWhileIdle(t *testing.T) {
	tr := newTestTracker(t, 2*time.Minute)

	focus(t, tr, "kitty", time.Now())
	tr.handleIdle(true)
	before := todayTotal(t, tr)

	tr.lastUpdate = time.Now().Add(-10 * time.Minute)
	tr.flush()
	tr.handleFocusChange("firefox")

	if got := todayTotal(t, tr); got != before {
		t.Fatalf("time counted while idle: %d -> %d", before, got)
	}
}

func TestResumeRestartsFromNow(t *testing.T) {
	tr := newTestTracker(t, 2*time.Minute)

	focus(t, tr, "kitty", time.Now())
	tr.handleIdle(true)
	tr.handleIdle(false)

	if tr.idle || time.Since(tr.lastUpdate) > time.Second {
		t.Fatalf("resume should reset lastUpdate to now")
	}
}
