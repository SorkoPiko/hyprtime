package db

import "time"

func (db *DB) GetOrCreateApp(class string) (int64, error) {
	if _, err := db.Exec(
		"INSERT INTO apps (class) VALUES (?) ON CONFLICT(class) DO NOTHING",
		class,
	); err != nil {
		return 0, err
	}

	var appID int64
	if err := db.QueryRow("SELECT id FROM apps WHERE class = ?", class).Scan(&appID); err != nil {
		return 0, err
	}

	return appID, nil
}

func (db *DB) UpdateAppTime(appID int64, duration int64) error {
	if duration <= 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	today := time.Now().Format("2006-01-02")
	_, err = tx.Exec(`
		INSERT INTO daily_usage (app_id, date, duration)
		VALUES (?, ?, ?)
		ON CONFLICT(app_id, date) DO UPDATE SET
			duration = duration + ?
	`, appID, today, duration, duration)
	if err != nil {
		return err
	}

	return tx.Commit()
}
