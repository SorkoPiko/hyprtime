package db

import "time"

func (db *DB) AppID(class string) (int64, error) {
	if _, err := db.db.Exec(
		"INSERT INTO apps (class) VALUES (?) ON CONFLICT(class) DO NOTHING",
		class,
	); err != nil {
		return 0, err
	}

	var id int64
	if err := db.db.QueryRow("SELECT id FROM apps WHERE class = ?", class).Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}

func (db *DB) AddUsage(appID, duration int64) error {
	if duration <= 0 {
		return nil
	}

	today := time.Now().Format("2006-01-02")

	_, err := db.db.Exec(`
		INSERT INTO daily_usage (app_id, date, duration)
		VALUES (?, ?, ?)
		ON CONFLICT(app_id, date) DO UPDATE SET
			duration = duration + excluded.duration
	`, appID, today, duration)

	return err
}
