package ui

import (
	"fmt"
	"time"
)

type Duration int64

func (d Duration) String() string {
	td := time.Duration(d) * time.Second
	h := int(td.Hours())
	m := int(td.Minutes()) % 60
	s := int(td.Seconds()) % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

type AppData struct {
	Class    string
	Duration Duration
}

type DailyData struct {
	Date          string
	TotalDuration Duration
	Apps          []AppData
}
