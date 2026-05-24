package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ayanrajpoot10/hyprtime/internal/sysutil"
)

//go:embed schema.sql
var schema string

type DB struct {
	*sql.DB
}

func New() (*DB, error) {
	dbPath, err := sysutil.DBPath()
	if err != nil {
		return nil, fmt.Errorf("db path: %w", err)
	}

	if err := sysutil.EnsureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("db dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db := &DB{sqlDB}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}
