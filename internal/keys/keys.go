// Package keys reads keyboards directly from /dev/input and classifies key
// presses with xkbcommon, using the same layout as the compositor. Only the
// kind of each key (character, separator, correction...) leaves this package.
package keys

/*
#cgo pkg-config: xkbcommon
#include <stdlib.h>
#include <xkbcommon/xkbcommon.h>
#include <xkbcommon/xkbcommon-keysyms.h>
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ayanrajpoot10/hyprtime/internal/logger"
	"github.com/ayanrajpoot10/hyprtime/internal/typing"
)

// Layout describes an XKB keymap by its RMLVO names.
type Layout struct {
	Rules, Model, Layout, Variant, Options string
	NumLock, CapsLock                      bool
}

const (
	evKey       = 0x01
	eventSize   = 24 // struct input_event on 64-bit
	scanEvery   = 3 * time.Second
	keyRelease  = 0
	keyPress    = 1
	keyRepeat   = 2
	evdevOffset = 8 // xkb keycodes are evdev codes + 8

	// Keys every real keyboard has; mice and headsets that register as
	// keyboards for their extra buttons don't
	keyEnter = 28
	keyA     = 30
	keyZ     = 44
	keySpace = 57
)

var (
	modCtrl = C.CString(C.XKB_MOD_NAME_CTRL)
	modAlt  = C.CString(C.XKB_MOD_NAME_ALT)
	modLogo = C.CString(C.XKB_MOD_NAME_LOGO)
	modNum  = C.CString(C.XKB_MOD_NAME_NUM)
	modCaps = C.CString(C.XKB_MOD_NAME_CAPS)
)

type device struct {
	path  string
	name  string
	file  *os.File
	state *C.struct_xkb_state
}

type Watcher struct {
	out chan typing.Key

	mu      sync.Mutex
	ctx     *C.struct_xkb_context
	keymap  *C.struct_xkb_keymap
	layout  Layout
	devices map[string]*device
	denied  bool
}

func NewWatcher() (*Watcher, error) {
	ctx := C.xkb_context_new(C.XKB_CONTEXT_NO_FLAGS)
	if ctx == nil {
		return nil, fmt.Errorf("create xkb context")
	}
	return &Watcher{
		out:     make(chan typing.Key, 256),
		ctx:     ctx,
		devices: map[string]*device{},
	}, nil
}

// Keys delivers classified key presses.
func (w *Watcher) Keys() <-chan typing.Key {
	return w.out
}

// SetLayout compiles the keymap and resets every keyboard's state to it.
func (w *Watcher) SetLayout(l Layout) error {
	names := C.struct_xkb_rule_names{
		rules:   C.CString(l.Rules),
		model:   C.CString(l.Model),
		layout:  C.CString(l.Layout),
		variant: C.CString(l.Variant),
		options: C.CString(l.Options),
	}
	defer func() {
		C.free(unsafe.Pointer(names.rules))
		C.free(unsafe.Pointer(names.model))
		C.free(unsafe.Pointer(names.layout))
		C.free(unsafe.Pointer(names.variant))
		C.free(unsafe.Pointer(names.options))
	}()

	keymap := C.xkb_keymap_new_from_names(w.ctx, &names, C.XKB_KEYMAP_COMPILE_NO_FLAGS)
	if keymap == nil {
		return fmt.Errorf("compile keymap %q %q", l.Layout, l.Variant)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.keymap != nil {
		C.xkb_keymap_unref(w.keymap)
	}
	w.keymap = keymap
	w.layout = l
	for _, d := range w.devices {
		w.resetState(d)
	}

	logger.Infof("Keymap: %s %s", l.Layout, l.Variant)
	return nil
}

// resetState gives a device a fresh state with the compositor's lock keys.
// Caller holds w.mu.
func (w *Watcher) resetState(d *device) {
	if d.state != nil {
		C.xkb_state_unref(d.state)
	}
	d.state = C.xkb_state_new(w.keymap)

	var locked C.xkb_mod_mask_t
	if w.layout.NumLock {
		if idx := C.xkb_keymap_mod_get_index(w.keymap, modNum); idx != C.XKB_MOD_INVALID {
			locked |= 1 << idx
		}
	}
	if w.layout.CapsLock {
		if idx := C.xkb_keymap_mod_get_index(w.keymap, modCaps); idx != C.XKB_MOD_INVALID {
			locked |= 1 << idx
		}
	}
	C.xkb_state_update_mask(d.state, 0, 0, locked, 0, 0, 0)
}

// Start begins scanning for keyboards. SetLayout must have been called.
func (w *Watcher) Start() {
	go func() {
		for {
			w.scan()
			time.Sleep(scanEvery)
		}
	}()
}

func (w *Watcher) scan() {
	paths, _ := filepath.Glob("/dev/input/event*")
	for _, path := range paths {
		w.mu.Lock()
		_, known := w.devices[path]
		w.mu.Unlock()
		if known {
			continue
		}

		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			if os.IsPermission(err) && !w.denied {
				w.denied = true
				logger.Errorf("No access to %s; typing stats need the user in the 'input' group", path)
			}
			continue
		}

		if !isKeyboard(f) {
			f.Close()
			continue
		}

		d := &device{path: path, name: deviceName(f), file: f}
		w.mu.Lock()
		w.resetState(d)
		w.devices[path] = d
		w.mu.Unlock()

		logger.Infof("Keyboard: %s (%s)", d.name, path)
		go w.read(d)
	}
}

func (w *Watcher) read(d *device) {
	buf := make([]byte, eventSize*64)
	for {
		n, err := d.file.Read(buf)
		if err != nil {
			logger.Infof("Keyboard gone: %s (%s)", d.name, d.path)
			w.mu.Lock()
			delete(w.devices, d.path)
			C.xkb_state_unref(d.state)
			w.mu.Unlock()
			d.file.Close()
			return
		}

		for off := 0; off+eventSize <= n; off += eventSize {
			ev := buf[off : off+eventSize]
			typ := binary.LittleEndian.Uint16(ev[16:18])
			if typ != evKey {
				continue
			}
			code := binary.LittleEndian.Uint16(ev[18:20])
			value := int32(binary.LittleEndian.Uint32(ev[20:24]))
			w.handle(d, code, value)
		}
	}
}

func (w *Watcher) handle(d *device, code uint16, value int32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	kc := C.xkb_keycode_t(uint32(code) + evdevOffset)

	// Classify with the modifiers held before this key changes them
	if value == keyPress || value == keyRepeat {
		if kind, ok := classify(d.state, kc); ok {
			select {
			case w.out <- typing.Key{Kind: kind, Time: time.Now()}:
			default:
				// Never block the reader; dropping one key under load is fine
			}
		}
	}

	switch value {
	case keyPress:
		C.xkb_state_update_key(d.state, kc, C.XKB_KEY_DOWN)
	case keyRelease:
		C.xkb_state_update_key(d.state, kc, C.XKB_KEY_UP)
	}
}

func modActive(state *C.struct_xkb_state, name *C.char) bool {
	return C.xkb_state_mod_name_is_active(state, name, C.XKB_STATE_MODS_EFFECTIVE) > 0
}

func classify(state *C.struct_xkb_state, kc C.xkb_keycode_t) (typing.Kind, bool) {
	sym := C.xkb_state_key_get_one_sym(state, kc)
	ctrl := modActive(state, modCtrl)
	alt := modActive(state, modAlt)
	logo := modActive(state, modLogo)

	if sym == C.XKB_KEY_BackSpace {
		switch {
		case logo:
			return 0, false
		case ctrl || alt:
			return typing.WordBackspace, true
		default:
			return typing.Backspace, true
		}
	}

	if ctrl || alt || logo {
		// Shortcuts aren't typing, and may change the text in unknown ways
		if isModifier(sym) {
			return 0, false
		}
		return typing.Reset, true
	}

	switch sym {
	case C.XKB_KEY_Return, C.XKB_KEY_KP_Enter, C.XKB_KEY_Tab, C.XKB_KEY_ISO_Left_Tab:
		return typing.Separator, true
	case C.XKB_KEY_Escape, C.XKB_KEY_Delete, C.XKB_KEY_KP_Delete,
		C.XKB_KEY_Left, C.XKB_KEY_Right, C.XKB_KEY_Up, C.XKB_KEY_Down,
		C.XKB_KEY_KP_Left, C.XKB_KEY_KP_Right, C.XKB_KEY_KP_Up, C.XKB_KEY_KP_Down,
		C.XKB_KEY_Home, C.XKB_KEY_End, C.XKB_KEY_Page_Up, C.XKB_KEY_Page_Down,
		C.XKB_KEY_KP_Home, C.XKB_KEY_KP_End, C.XKB_KEY_KP_Page_Up, C.XKB_KEY_KP_Page_Down:
		return typing.Reset, true
	}

	cp := C.xkb_state_key_get_utf32(state, kc)
	switch {
	case cp == ' ':
		return typing.Separator, true
	case cp >= 0x20 && cp != 0x7f:
		return typing.Char, true
	}
	return 0, false
}

func isModifier(sym C.xkb_keysym_t) bool {
	return (sym >= C.XKB_KEY_Shift_L && sym <= C.XKB_KEY_Hyper_R) ||
		(sym >= C.XKB_KEY_ISO_Lock && sym <= C.XKB_KEY_ISO_Level5_Lock)
}

func ioc(nr, size uintptr) uintptr {
	const read = 2
	return read<<30 | size<<16 | 'E'<<8 | nr
}

func isKeyboard(f *os.File) bool {
	var bits [96]byte                          // KEY_MAX (0x2ff) bits
	req := ioc(0x20+evKey, uintptr(len(bits))) // EVIOCGBIT(EV_KEY)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(unsafe.Pointer(&bits[0]))); errno != 0 {
		return false
	}
	has := func(code int) bool { return bits[code/8]&(1<<(code%8)) != 0 }
	return has(keyA) && has(keyZ) && has(keySpace) && has(keyEnter)
}

func deviceName(f *os.File) string {
	var name [256]byte
	req := ioc(0x06, uintptr(len(name))) // EVIOCGNAME
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(unsafe.Pointer(&name[0]))); errno != 0 {
		return "unknown"
	}
	for i, b := range name {
		if b == 0 {
			return string(name[:i])
		}
	}
	return string(name[:])
}
