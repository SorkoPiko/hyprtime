package db

import "time"

type TypingTotals struct {
	Chars      int64
	Backspaces int64
	Words      int64
	ActiveMs   int64
}

type Burst struct {
	Start      time.Time
	AppID      int64
	DurationMs int64
	Chars      int64
	Backspaces int64
}

// BurstTotals aggregates the bursts of one day, for typing speed.
type BurstTotals struct {
	Chars      int64
	Backspaces int64
	DurationMs int64
	BestWPM    float64
}

func (db *DB) AddTyping(appID int64, t TypingTotals) error {
	if t == (TypingTotals{}) {
		return nil
	}

	today := time.Now().Format("2006-01-02")
	_, err := db.db.Exec(`
		INSERT INTO typing_daily (app_id, date, chars, backspaces, words, active_ms)
		VALUES (?, ?, ?, ?, MAX(0, ?), ?)
		ON CONFLICT(app_id, date) DO UPDATE SET
			chars = chars + excluded.chars,
			backspaces = backspaces + excluded.backspaces,
			words = MAX(0, words + ?),
			active_ms = active_ms + excluded.active_ms
	`, appID, today, t.Chars, t.Backspaces, t.Words, t.ActiveMs, t.Words)
	return err
}

func (db *DB) AddBurst(b Burst) error {
	_, err := db.db.Exec(`
		INSERT INTO typing_bursts (ts, date, app_id, duration_ms, chars, backspaces)
		VALUES (?, ?, ?, ?, ?, ?)
	`, b.Start.UnixMilli(), b.Start.Format("2006-01-02"), b.AppID, b.DurationMs, b.Chars, b.Backspaces)
	return err
}

func (db *DB) TypingByDate(date string) (TypingTotals, error) {
	var t TypingTotals
	err := db.db.QueryRow(`
		SELECT COALESCE(SUM(chars), 0), COALESCE(SUM(backspaces), 0),
		       COALESCE(SUM(words), 0), COALESCE(SUM(active_ms), 0)
		FROM typing_daily WHERE date = ?
	`, date).Scan(&t.Chars, &t.Backspaces, &t.Words, &t.ActiveMs)
	return t, err
}

func (db *DB) BurstsByDate(date string) (BurstTotals, error) {
	var b BurstTotals
	err := db.db.QueryRow(`
		SELECT COALESCE(SUM(chars), 0), COALESCE(SUM(backspaces), 0), COALESCE(SUM(duration_ms), 0),
		       COALESCE(MAX(MAX(chars - backspaces, 0) / 5.0 / (duration_ms / 60000.0)), 0)
		FROM typing_bursts WHERE date = ? AND duration_ms > 0
	`, date).Scan(&b.Chars, &b.Backspaces, &b.DurationMs, &b.BestWPM)
	return b, err
}
