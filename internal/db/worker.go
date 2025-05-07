package db

import (
	"database/sql"
)

func ProcessPending(db *sql.DB) error {
	trades, err := FetchPendingTrades(db)
	if err != nil {
		return err
	}
	for _, t := range trades {
		profit := (t.Close - t.Open) * t.Volume * 100000.0
		if t.Side == "sell" {
			profit = -profit
		}
		if err := UpdateStats(db, t.Account, profit); err != nil {
			return err
		}
		if err := MarkProcessed(db, t.ID); err != nil {
			return err
		}
	}
	return nil
}
