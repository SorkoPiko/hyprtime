package editor

import (
	"fmt"
	"os"
	"testing"
)

func TestSuppressedOnlyForFocusedCommandModes(t *testing.T) {
	tr := &Tracker{instances: map[int]instance{}}
	pid := os.Getpid() // a live process

	cases := []struct {
		line string
		want bool
	}{
		{fmt.Sprintf("%d 1 n", pid), true},
		{fmt.Sprintf("%d 1 i", pid), false},
		{fmt.Sprintf("%d 1 Rv", pid), false},
		{fmt.Sprintf("%d 1 c", pid), true},
		{fmt.Sprintf("%d 1 t", pid), false},
		{fmt.Sprintf("%d 0 n", pid), false},
		{fmt.Sprintf("%d 1 n", pid), true},
		{fmt.Sprintf("%d 0 exit", pid), false},
	}
	for _, c := range cases {
		tr.report(c.line)
		if got := tr.Suppressed(); got != c.want {
			t.Fatalf("%q: suppressed=%v, want %v", c.line, got, c.want)
		}
	}
}

func TestDeadEditorIsForgotten(t *testing.T) {
	tr := &Tracker{instances: map[int]instance{}}
	// PIDs above pid_max never exist
	tr.report("99999999 1 n")
	if tr.Suppressed() {
		t.Fatal("dead editor should not suppress typing")
	}
	if len(tr.instances) != 0 {
		t.Fatal("dead editor should be removed")
	}
}

func TestNilTrackerNeverSuppresses(t *testing.T) {
	var tr *Tracker
	if tr.Suppressed() {
		t.Fatal("nil tracker suppressed")
	}
}
