package db

import (
	"database/sql"
)

func InitDB(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS trades_q (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            account TEXT NOT NULL,
            symbol TEXT NOT NULL,
            volume REAL NOT NULL,
            open REAL NOT NULL,
            close REAL NOT NULL,
            side TEXT NOT NULL,
            processed INTEGER NOT NULL DEFAULT 0
        );`,
		`CREATE TABLE IF NOT EXISTS account_stats (
            account TEXT PRIMARY KEY,
            trades INTEGER NOT NULL DEFAULT 0,
            profit REAL NOT NULL DEFAULT 0
        );`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
