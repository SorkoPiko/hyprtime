// hyprtimectl reads live hyprtime state for status bars.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/typing"
)

const usage = `usage: hyprtimectl waybar [--follow]

  waybar     print typing stats as a waybar custom module JSON line
  --follow   keep running and print a new line whenever the output changes
`

type waybarOutput struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class"`
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != "waybar" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	follow := len(args) > 1 && args[1] == "--follow"

	if !follow {
		printLine(waybar(time.Now()))
		return
	}

	var last string
	for {
		line := waybar(time.Now())
		if line != last {
			printLine(line)
			last = line
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func printLine(line string) {
	fmt.Println(line)
}

func waybar(now time.Time) string {
	out := render(now)
	data, _ := json.Marshal(out)
	return string(data)
}

func render(now time.Time) waybarOutput {
	s, err := typing.ReadState()
	if errors.Is(err, fs.ErrNotExist) {
		return waybarOutput{Text: "– wpm ", Tooltip: "hyprtimed isn't running", Class: "no-data"}
	}
	if err != nil {
		return waybarOutput{Text: "– wpm ", Tooltip: err.Error(), Class: "no-data"}
	}
	if s.Date != now.Format("2006-01-02") {
		return waybarOutput{Text: "– wpm ", Tooltip: "Nothing typed today", Class: "no-data"}
	}

	avg := typing.WPM(s.BurstChars, s.BurstBack, s.BurstMs)
	typingNow := s.Typing && now.Sub(time.UnixMilli(s.LastKeyMs)) <= typing.BurstGap

	out := waybarOutput{Tooltip: tooltip(s, avg)}
	switch {
	case typingNow:
		wpm := s.LiveWPM
		if wpm == 0 {
			wpm = s.LastWPM
		}
		out.Text = fmt.Sprintf("%.0f wpm ", wpm)
		out.Class = "typing"
	case s.BurstMs > 0:
		out.Text = fmt.Sprintf("%.0f wpm ", avg)
		out.Class = "idle"
	default:
		out.Text = "– wpm "
		out.Class = "no-data"
	}
	return out
}

func tooltip(s typing.Snapshot, avg float64) string {
	rows := [][2]string{
		{"Words", thousands(s.Words)},
		{"Characters", thousands(s.Chars)},
		{"Active", duration(s.ActiveMs)},
	}
	if s.BurstMs > 0 {
		rows = append(rows,
			[2]string{"Average", fmt.Sprintf("%.0f wpm · %.0f cpm", avg, avg*5)},
			[2]string{"Accuracy", fmt.Sprintf("%.1f%%", typing.Accuracy(s.BurstChars, s.BurstBack))},
			[2]string{"Best burst", fmt.Sprintf("%.0f wpm", s.BestWPM)},
		)
	}
	if s.LastWPM > 0 {
		rows = append(rows, [2]string{"Last burst", fmt.Sprintf("%.0f wpm · %.1f%%", s.LastWPM, s.LastAcc)})
	}

	lines := []string{"<b>Typing today</b>"}
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("<tt>%-11s</tt>%s", r[0], r[1]))
	}
	return strings.Join(lines, "\n")
}

func thousands(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func duration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
