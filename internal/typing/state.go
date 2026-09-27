package typing

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ayanrajpoot10/hyprtime/internal/rundir"
)

func StatePath() string {
	return rundir.Path("typing.json")
}

// WriteState publishes a snapshot atomically for status bar readers.
func WriteState(s Snapshot) error {
	path := StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ReadState() (Snapshot, error) {
	var s Snapshot
	data, err := os.ReadFile(StatePath())
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}
