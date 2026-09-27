CREATE TABLE IF NOT EXISTS apps (
	id INTEGER PRIMARY KEY,
	class TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS daily_usage (
	app_id INTEGER NOT NULL,
	date TEXT NOT NULL,
	duration INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (app_id, date),
	FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(date);

-- Typing activity: counts only, never the keys or text themselves
CREATE TABLE IF NOT EXISTS typing_daily (
	app_id INTEGER NOT NULL,
	date TEXT NOT NULL,
	chars INTEGER NOT NULL DEFAULT 0,
	backspaces INTEGER NOT NULL DEFAULT 0,
	words INTEGER NOT NULL DEFAULT 0,
	active_ms INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (app_id, date),
	FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_typing_daily_date ON typing_daily(date);

-- One row per continuous typing burst, for speed history
CREATE TABLE IF NOT EXISTS typing_bursts (
	id INTEGER PRIMARY KEY,
	ts INTEGER NOT NULL,
	date TEXT NOT NULL,
	app_id INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL,
	chars INTEGER NOT NULL,
	backspaces INTEGER NOT NULL,
	FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_typing_bursts_date ON typing_bursts(date);
