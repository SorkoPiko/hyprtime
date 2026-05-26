package db

type AppUsage struct {
	AppID    int64
	Class    string
	Duration int64
}

func (db *DB) UsageByDate(date string) ([]AppUsage, error) {
	rows, err := db.db.Query(`
		SELECT du.app_id, a.class, du.duration
		FROM daily_usage du
		JOIN apps a ON du.app_id = a.id
		WHERE du.date = ?
		ORDER BY du.duration DESC
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usage []AppUsage
	for rows.Next() {
		var u AppUsage
		if err := rows.Scan(&u.AppID, &u.Class, &u.Duration); err != nil {
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
