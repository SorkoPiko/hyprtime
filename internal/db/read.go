package db

type DailyUsage struct {
	AppID     int64  `json:"app_id"`
	Class     string `json:"class"`
	Date      string `json:"date"`
	TotalTime int64  `json:"total_time"`
}

func (db *DB) GetDailyUsage(date string) ([]DailyUsage, error) {
	rows, err := db.db.Query(`
		SELECT ds.app_id, a.class, ds.date, ds.duration
		FROM daily_usage ds
		JOIN apps a ON ds.app_id = a.id
		WHERE ds.date = ?
		ORDER BY ds.duration DESC
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DailyUsage
	for rows.Next() {
		var s DailyUsage
		if err := rows.Scan(&s.AppID, &s.Class, &s.Date, &s.TotalTime); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

func (db *DB) GetTotalScreenTimeForDate(date string) (int64, error) {
	var total int64
	err := db.db.QueryRow(`
		SELECT COALESCE(SUM(duration), 0)
		FROM daily_usage
		WHERE date = ?
	`, date).Scan(&total)
	return total, err
}
