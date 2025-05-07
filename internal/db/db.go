package db

import (
	"database/sql"
)

type Trade struct {
	ID      int
	Account string
	Symbol  string
	Volume  float64
	Open    float64
	Close   float64
	Side    string
}

type Stats struct {
	Account string
	Trades  int
	Profit  float64
}

func EnqueueTrade(db *sql.DB, t Trade) error {
	_, err := db.Exec(
		`INSERT INTO trades_q (account, symbol, volume, open, close, side) VALUES (?, ?, ?, ?, ?, ?)`,
		t.Account, t.Symbol, t.Volume, t.Open, t.Close, t.Side,
	)
	return err
}

func FetchPendingTrades(db *sql.DB) ([]Trade, error) {
	rows, err := db.Query(
		`SELECT id, account, symbol, volume, open, close, side FROM trades_q WHERE processed = 0`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		if err := rows.Scan(&t.ID, &t.Account, &t.Symbol, &t.Volume, &t.Open, &t.Close, &t.Side); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

func MarkProcessed(db *sql.DB, id int) error {
	_, err := db.Exec(
		`UPDATE trades_q SET processed = 1 WHERE id = ?`,
		id,
	)
	return err
}

func UpdateStats(db *sql.DB, account string, profit float64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO account_stats (account, trades, profit) VALUES (?, 1, ?)
		ON CONFLICT(account) DO UPDATE SET trades = trades + 1, profit = profit + ?`,
		account, profit, profit,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func GetStats(db *sql.DB, account string) (Stats, error) {
	var s Stats
	s.Account = account

	r := db.QueryRow(
		`SELECT trades, profit FROM account_stats WHERE account = ?`,
		account,
	)
	if err := r.Scan(&s.Trades, &s.Profit); err != nil {
		if err == sql.ErrNoRows {
			return s, nil
		}
		return s, err
	}
	return s, nil
}
