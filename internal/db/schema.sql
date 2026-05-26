CREATE TABLE IF NOT EXISTS apps (
	id INTEGER PRIMARY KEY,
	class TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS daily_usage (
	app_id INTEGER NOT NULL,
	date TEXT NOT NULL,
	duration INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (app_id, date),
	FOREIGN KEY (app_id) REFERENCES apps(id)
);

CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(date);
