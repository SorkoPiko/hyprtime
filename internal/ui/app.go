package ui

import (
	"fmt"
	"log"
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/db"

	"github.com/gotk3/gotk3/gtk"
)

type App struct {
	db             *db.DB
	today          time.Time
	appList        *gtk.ListBox
	totalTimeLabel *gtk.Label
	dateLabel      *gtk.Label
}

func New(db *db.DB) *App {
	return &App{
		db:    db,
		today: time.Now(),
	}
}

func (a *App) formatDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func (a *App) dailyUsage(date string) (*DailyData, error) {
	usage, err := a.db.UsageByDate(date)
	if err != nil {
		return nil, fmt.Errorf("db error: %w", err)
	}

	totalTime, err := a.db.TotalDurationByDate(date)
	if err != nil {
		return nil, fmt.Errorf("db error: %w", err)
	}

	apps := make([]AppData, 0, len(usage))
	for _, app := range usage {
		percentage := 0.0
		if totalTime > 0 {
			percentage = float64(app.Duration) / float64(totalTime) * 100
		}

		apps = append(apps, AppData{
			Class:              app.Class,
			TotalTime:          app.Duration,
			TotalTimeFormatted: a.formatDuration(app.Duration),
			Percentage:         percentage,
		})
	}

	return &DailyData{
		Date:               date,
		TotalTime:          totalTime,
		TotalTimeFormatted: a.formatDuration(totalTime),
		Apps:               apps,
	}, nil
}

func (a *App) Run() error {
	gtk.Init(nil)

	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("create window: %w", err)
	}
	win.SetTitle("Hyprtime")
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	mainBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	win.Add(mainBox)

	headerBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	headerBox.SetMarginStart(10)
	headerBox.SetMarginEnd(10)
	headerBox.SetMarginTop(10)
	headerBox.SetHAlign(gtk.ALIGN_CENTER)

	prevBtn, _ := gtk.ButtonNewWithLabel("<")
	prevBtn.Connect("clicked", func() {
		a.today = a.today.AddDate(0, 0, -1)
		a.updateData()
	})

	a.dateLabel, _ = gtk.LabelNew(a.today.Format("2006-01-02"))

	nextBtn, _ := gtk.ButtonNewWithLabel(">")
	nextBtn.Connect("clicked", func() {
		if a.today.Format("2006-01-02") >= time.Now().Format("2006-01-02") {
			return
		}

		a.today = a.today.AddDate(0, 0, 1)
		a.updateData()
	})

	headerBox.PackStart(prevBtn, false, false, 0)
	headerBox.PackStart(a.dateLabel, false, false, 10)
	headerBox.PackStart(nextBtn, false, false, 0)
	mainBox.PackStart(headerBox, false, false, 0)

	a.totalTimeLabel, _ = gtk.LabelNew("Total Time: 0s")
	a.totalTimeLabel.SetMarkup("<b>Total Time: 0s</b>")
	mainBox.PackStart(a.totalTimeLabel, false, false, 10)

	scrolledWindow, _ := gtk.ScrolledWindowNew(nil, nil)
	scrolledWindow.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	mainBox.PackStart(scrolledWindow, true, true, 0)

	a.appList, _ = gtk.ListBoxNew()
	a.appList.SetSelectionMode(gtk.SELECTION_NONE)
	scrolledWindow.Add(a.appList)

	a.updateData()

	win.ShowAll()
	gtk.Main()

	return nil
}

func (a *App) updateData() {
	date := a.today.Format("2006-01-02")
	a.dateLabel.SetText(date)

	stats, err := a.dailyUsage(date)
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		a.totalTimeLabel.SetMarkup("<span size=\"xx-large\"><b>0s</b></span>")
		a.clearList()
		return
	}

	a.totalTimeLabel.SetMarkup(fmt.Sprintf("<span size=\"xx-large\"><b>%s</b></span>", stats.TotalTimeFormatted))

	a.clearList()

	sortedApps := make([]AppData, len(stats.Apps))
	copy(sortedApps, stats.Apps)
	for i := 0; i < len(sortedApps); i++ {
		for j := i + 1; j < len(sortedApps); j++ {
			if sortedApps[j].TotalTime > sortedApps[i].TotalTime {
				sortedApps[i], sortedApps[j] = sortedApps[j], sortedApps[i]
			}
		}
	}

	for _, app := range sortedApps {
		rowBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
		rowBox.SetMarginStart(15)
		rowBox.SetMarginEnd(15)
		rowBox.SetMarginTop(8)
		rowBox.SetMarginBottom(8)

		headerBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)

		appNameLabel, _ := gtk.LabelNew(app.Class)
		appNameLabel.SetMarkup(fmt.Sprintf("<b>%s</b>", app.Class))
		appNameLabel.SetXAlign(0)
		headerBox.PackStart(appNameLabel, true, true, 0)

		percentLabel, _ := gtk.LabelNew(fmt.Sprintf("%.1f%%", app.Percentage))
		percentLabel.SetMarkup(fmt.Sprintf("<span size=\"small\">%.1f%%</span>", app.Percentage))
		headerBox.PackEnd(percentLabel, false, false, 0)

		timeLabel, _ := gtk.LabelNew(app.TotalTimeFormatted)
		timeLabel.SetXAlign(0)
		timeLabel.SetMarkup(fmt.Sprintf("<span size=\"small\" foreground=\"#666\">%s</span>", app.TotalTimeFormatted))

		progressBar, _ := gtk.ProgressBarNew()
		progressBar.SetFraction(app.Percentage / 100.0)
		progressBar.SetShowText(false)

		rowBox.Add(headerBox)
		rowBox.Add(progressBar)
		rowBox.Add(timeLabel)

		a.appList.Add(rowBox)
	}
	a.appList.ShowAll()
}

func (a *App) clearList() {
	a.appList.GetChildren().Foreach(func(item any) {
		widget := item.(*gtk.Widget)
		a.appList.Remove(widget)
		widget.Destroy()
	})
}
