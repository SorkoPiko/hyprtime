package typing

import (
	"testing"
	"time"
)

const app = 1

// typer feeds keys to a counter with a fixed interval between them.
type typer struct {
	c   *Counter
	now time.Time
}

func newTyper() *typer {
	return &typer{c: NewCounter(), now: time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)}
}

func (ty *typer) key(k Kind) {
	ty.now = ty.now.Add(200 * time.Millisecond)
	ty.c.Handle(Key{Kind: k, Time: ty.now}, app)
}

// text types s, with ' ' and '\n' as separators and '<' as backspace.
func (ty *typer) text(s string) {
	for _, r := range s {
		switch r {
		case ' ', '\n':
			ty.key(Separator)
		case '<':
			ty.key(Backspace)
		default:
			ty.key(Char)
		}
	}
}

func (ty *typer) pause(d time.Duration) {
	ty.now = ty.now.Add(d)
	ty.c.Tick(ty.now)
}

func (ty *typer) totals() (chars, back, words int64) {
	s := ty.c.Snapshot(ty.now)
	return s.Chars, s.Backspaces, s.Words
}

func TestCountsCharsAndWords(t *testing.T) {
	ty := newTyper()
	ty.text("hello there world ")

	chars, back, words := ty.totals()
	if chars != 18 || back != 0 || words != 3 {
		t.Fatalf("got chars=%d back=%d words=%d", chars, back, words)
	}
}

func TestUnfinishedWordNotCounted(t *testing.T) {
	ty := newTyper()
	ty.text("hello wor")

	if _, _, words := ty.totals(); words != 1 {
		t.Fatalf("want 1 word, got %d", words)
	}
}

func TestMultipleSeparatorsCountOnce(t *testing.T) {
	ty := newTyper()
	ty.text("one  \n\ntwo ")

	if _, _, words := ty.totals(); words != 2 {
		t.Fatalf("want 2 words, got %d", words)
	}
}

func TestBackspaceIntoWordUncompletesIt(t *testing.T) {
	ty := newTyper()
	// Type a word and a space, delete the space and a letter, retype
	ty.text("hello <<p ")

	chars, back, words := ty.totals()
	if words != 1 {
		t.Fatalf("want 1 word after correction, got %d", words)
	}
	if chars != 8 || back != 2 {
		t.Fatalf("got chars=%d back=%d", chars, back)
	}
}

func TestWordBackspaceDeletesWordAndGap(t *testing.T) {
	ty := newTyper()
	ty.text("keep this ")
	ty.key(WordBackspace)

	_, back, words := ty.totals()
	if back != 5 {
		t.Fatalf("want 5 deleted (\"this \"), got %d", back)
	}
	if words != 1 {
		t.Fatalf("want 1 word left, got %d", words)
	}
}

func TestWordBackspaceWithoutHistory(t *testing.T) {
	ty := newTyper()
	ty.key(Reset)
	ty.key(WordBackspace)

	if _, back, _ := ty.totals(); back != unknownWordLen {
		t.Fatalf("want %d, got %d", unknownWordLen, back)
	}
}

func TestResetStopsWordCorrections(t *testing.T) {
	ty := newTyper()
	ty.text("one ")
	// Caret moved; the backspace deletes something we can't see
	ty.key(Reset)
	ty.text("<")

	if _, _, words := ty.totals(); words != 1 {
		t.Fatalf("word should stay counted after a reset, got %d", words)
	}
}

func TestBurstSpeedAndAccuracy(t *testing.T) {
	ty := newTyper()
	// 20 keys at 200ms: 19 intervals = 3.8s; 18 net chars
	ty.text("abcdefghijklmnopq<<")
	ty.text("ab")
	ty.pause(3 * time.Second)

	s := ty.c.Snapshot(ty.now)
	if s.Typing {
		t.Fatalf("burst should have ended")
	}
	// 21 keys: 19 chars, 2 backspaces, 4s -> 17 net / 5 / (4/60) = 51 wpm
	if s.LastWPM != 51 {
		t.Fatalf("want 51 wpm, got %v", s.LastWPM)
	}
	if s.LastAcc < 89.4 || s.LastAcc > 89.5 {
		t.Fatalf("want ~89.5%% accuracy, got %v", s.LastAcc)
	}
	if s.ActiveMs != 4000 || s.BurstMs != 4000 {
		t.Fatalf("active=%d burst=%d", s.ActiveMs, s.BurstMs)
	}
}

func TestShortBurstNotUsedForSpeed(t *testing.T) {
	ty := newTyper()
	ty.text("ok ")
	ty.pause(3 * time.Second)

	_, bursts := ty.c.Drain()
	if len(bursts) != 0 {
		t.Fatalf("short burst stored")
	}
	if s := ty.c.Snapshot(ty.now); s.ActiveMs == 0 {
		t.Fatalf("short burst should still count as active time")
	}
}

func TestPauseSplitsBursts(t *testing.T) {
	ty := newTyper()
	ty.text("first burst text ")
	ty.pause(5 * time.Second)
	ty.text("second burst here ")
	ty.pause(5 * time.Second)

	_, bursts := ty.c.Drain()
	if len(bursts) != 2 {
		t.Fatalf("want 2 bursts, got %d", len(bursts))
	}
}

func TestFocusChangeEndsBurst(t *testing.T) {
	ty := newTyper()
	ty.text("typing in one app ")
	ty.now = ty.now.Add(200 * time.Millisecond)
	ty.c.Handle(Key{Kind: Char, Time: ty.now}, 2)

	totals, bursts := ty.c.Drain()
	if len(bursts) != 1 || bursts[0].AppID != app {
		t.Fatalf("burst should end and belong to the first app: %+v", bursts)
	}
	if totals[2].Chars != 1 {
		t.Fatalf("second app should have 1 char, got %+v", totals[2])
	}
}

func TestDrainEmptiesPending(t *testing.T) {
	ty := newTyper()
	ty.text("some words here ")
	ty.pause(3 * time.Second)

	totals, _ := ty.c.Drain()
	if totals[app].Chars != 16 || totals[app].Words != 3 {
		t.Fatalf("got %+v", totals[app])
	}
	if totals, bursts := ty.c.Drain(); len(totals) != 0 || len(bursts) != 0 {
		t.Fatalf("second drain should be empty")
	}
	// Today's totals survive draining
	if s := ty.c.Snapshot(ty.now); s.Chars != 16 {
		t.Fatalf("today's totals lost: %+v", s)
	}
}

func TestDayRollover(t *testing.T) {
	ty := newTyper()
	ty.text("yesterday ")
	ty.now = ty.now.Add(24 * time.Hour)
	ty.text("today ")

	if chars, _, words := ty.totals(); chars != 6 || words != 1 {
		t.Fatalf("today should start fresh: chars=%d words=%d", chars, words)
	}
}

func TestLiveWPMWhileTyping(t *testing.T) {
	ty := newTyper()
	ty.text("abcdefghijk")

	s := ty.c.Snapshot(ty.now)
	if !s.Typing || s.LiveWPM <= 0 {
		t.Fatalf("want live wpm while typing: %+v", s)
	}
}
