package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

type DB struct {
	db *sql.DB
}

func New() (*DB, error) {
	dbPath, err := dbPath()
	if err != nil {
		return nil, fmt.Errorf("db path: %w", err)
	}

	if err := ensureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("db dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db := &DB{db: sqlDB}

	if _, err := db.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.db.Exec(schema); err != nil {
		db.db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.db.Close()
}
