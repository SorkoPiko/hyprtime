// Package rundir locates hyprtime's per-session runtime directory.
package rundir

import (
	"fmt"
	"os"
	"path/filepath"
)

// Path joins name onto $XDG_RUNTIME_DIR/hyprtime.
func Path(name string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(dir, "hyprtime", name)
}
