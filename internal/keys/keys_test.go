package keys

import (
	"testing"

	"github.com/ayanrajpoot10/hyprtime/internal/typing"
)

// evdev key codes
const (
	kBackspace = 14
	kQ         = 16
	kEnter     = 28
	kLeftCtrl  = 29
	kA         = 30
	kC         = 46
	kLeftShift = 42
	kSemicolon = 39
	kSpace     = 57
	kCapsLock  = 58
	kKP1       = 79
	kLeftMeta  = 125
	kLeft      = 105
	kF5        = 63
)

func newTestDevice(t *testing.T, l Layout) (*Watcher, *device) {
	t.Helper()
	w, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SetLayout(l); err != nil {
		t.Fatal(err)
	}
	d := &device{path: "test"}
	w.mu.Lock()
	w.resetState(d)
	w.mu.Unlock()
	return w, d
}

func press(w *Watcher, d *device, code uint16) {
	w.handle(d, code, keyPress)
}

func release(w *Watcher, d *device, code uint16) {
	w.handle(d, code, keyRelease)
}

// tap presses and releases a key and returns what it was classified as.
func tap(t *testing.T, w *Watcher, d *device, code uint16) (typing.Kind, bool) {
	t.Helper()
	press(w, d, code)
	release(w, d, code)
	select {
	case k := <-w.out:
		return k.Kind, true
	default:
		return 0, false
	}
}

func expect(t *testing.T, w *Watcher, d *device, code uint16, want typing.Kind) {
	t.Helper()
	got, ok := tap(t, w, d, code)
	if !ok {
		t.Fatalf("key %d: nothing emitted, want %v", code, want)
	}
	if got != want {
		t.Fatalf("key %d: got %v, want %v", code, got, want)
	}
}

func expectNothing(t *testing.T, w *Watcher, d *device, code uint16) {
	t.Helper()
	if got, ok := tap(t, w, d, code); ok {
		t.Fatalf("key %d: want nothing, got %v", code, got)
	}
}

var colemakDH = Layout{Layout: "us", Variant: "colemak_dh", NumLock: true}

func TestPlainKeys(t *testing.T) {
	w, d := newTestDevice(t, colemakDH)

	expect(t, w, d, kA, typing.Char)
	expect(t, w, d, kSemicolon, typing.Char) // 'o' on colemak
	expect(t, w, d, kSpace, typing.Separator)
	expect(t, w, d, kEnter, typing.Separator)
	expect(t, w, d, kBackspace, typing.Backspace)
	expect(t, w, d, kLeft, typing.Reset)
	expectNothing(t, w, d, kF5)
}

func TestShiftedCharsCount(t *testing.T) {
	w, d := newTestDevice(t, colemakDH)

	press(w, d, kLeftShift)
	expect(t, w, d, kQ, typing.Char)
	release(w, d, kLeftShift)
}

func TestShortcuts(t *testing.T) {
	w, d := newTestDevice(t, colemakDH)

	press(w, d, kLeftCtrl)
	expect(t, w, d, kBackspace, typing.WordBackspace)
	expect(t, w, d, kC, typing.Reset)
	release(w, d, kLeftCtrl)

	press(w, d, kLeftMeta)
	expect(t, w, d, kA, typing.Reset)
	release(w, d, kLeftMeta)

	// Back to normal once released
	expect(t, w, d, kA, typing.Char)
}

func TestModifiersAloneEmitNothing(t *testing.T) {
	w, d := newTestDevice(t, colemakDH)

	expectNothing(t, w, d, kLeftShift)
	expectNothing(t, w, d, kLeftCtrl)
}

func TestCapsLockFollowsLayout(t *testing.T) {
	// colemak_dh keeps Caps Lock as Caps Lock
	w, d := newTestDevice(t, colemakDH)
	expectNothing(t, w, d, kCapsLock)

	// plain colemak maps it to BackSpace
	w, d = newTestDevice(t, Layout{Layout: "us", Variant: "colemak"})
	expect(t, w, d, kCapsLock, typing.Backspace)
}

func TestNumpadFollowsNumLock(t *testing.T) {
	w, d := newTestDevice(t, colemakDH)
	expect(t, w, d, kKP1, typing.Char)

	w, d = newTestDevice(t, Layout{Layout: "us", Variant: "colemak_dh", NumLock: false})
	expect(t, w, d, kKP1, typing.Reset) // KP_End
}

func TestRepeatCountsWithoutChangingState(t *testing.T) {
	w, d := newTestDevice(t, colemakDH)

	press(w, d, kBackspace)
	w.handle(d, kBackspace, keyRepeat)
	w.handle(d, kBackspace, keyRepeat)
	release(w, d, kBackspace)

	if n := len(w.out); n != 3 {
		t.Fatalf("want 3 backspaces from press + 2 repeats, got %d", n)
	}
}
