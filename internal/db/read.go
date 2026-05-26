package db

type AppUsage struct {
	AppID    int64
	Class    string
	Date     string
	Duration int64
}

func (db *DB) UsageByDate(date string) ([]AppUsage, error) {
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

	var usage []AppUsage
	for rows.Next() {
		var u AppUsage
		if err := rows.Scan(&u.AppID, &u.Class, &u.Date, &u.Duration); err != nil {
			return nil, err
		}
		usage = append(usage, u)
	}

	return usage, rows.Err()
}

func (db *DB) TotalDurationByDate(date string) (int64, error) {
	var total int64
	err := db.db.QueryRow(`
		SELECT COALESCE(SUM(duration), 0)
		FROM daily_usage
		WHERE date = ?
	`, date).Scan(&total)
	return total, err
}
