package ui

type AppData struct {
	Class              string
	TotalTime          int64
	TotalTimeFormatted string
	Percentage         float64
}

type DailyData struct {
	Date               string
	TotalTime          int64
	TotalTimeFormatted string
	Apps               []AppData
}
