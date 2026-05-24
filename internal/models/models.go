package models

type AppData struct {
	Class              string  `json:"class"`
	TotalTime          int64   `json:"total_time"`
	TotalTimeFormatted string  `json:"total_time_formatted"`
	Percentage         float64 `json:"percentage"`
}

type DailyData struct {
	Date               string    `json:"date"`
	TotalTime          int64     `json:"total_time"`
	TotalTimeFormatted string    `json:"total_time_formatted"`
	Apps               []AppData `json:"apps"`
}
