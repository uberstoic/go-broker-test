package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	dbm "gitlab.com/digineat/go-broker-test/internal/db"
)

func InitWorkerDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	if err := dbm.InitDB(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	return db, nil
}

func CalculateProfitFromTrade(t dbm.Trade) float64 {
	profit := (t.Close - t.Open) * t.Volume * 100000.0
	if t.Side == "sell" {
		profit = -profit
	}
	return profit
}

func ProcessTrade(db *sql.DB, t dbm.Trade) error {
	profit := CalculateProfitFromTrade(t)

	if err := dbm.UpdateStats(db, t.Account, profit); err != nil {
		return fmt.Errorf("error updating stats for trade %d: %v", t.ID, err)
	}

	if err := dbm.MarkProcessed(db, t.ID); err != nil {
		return fmt.Errorf("error marking trade %d processed: %v", t.ID, err)
	}

	return nil
}

func ProcessPendingTrades(db *sql.DB) (int, error) {
	trades, err := dbm.FetchPendingTrades(db)
	if err != nil {
		return 0, fmt.Errorf("error fetching trades: %v", err)
	}

	processedCount := 0
	for _, t := range trades {
		err := ProcessTrade(db, t)
		if err != nil {
			log.Printf("%v", err)
		} else {
			processedCount++
		}
	}

	return processedCount, nil
}

func RunWorker(db *sql.DB, pollInterval time.Duration, stopChan <-chan struct{}) {
	log.Printf("Worker started with polling interval: %v", pollInterval)

	timer := time.NewTicker(pollInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			processedCount, err := ProcessPendingTrades(db)
			if err != nil {
				log.Printf("%v", err)
			} else if processedCount > 0 {
				log.Printf("Processed %d trades", processedCount)
			}
		case <-stopChan:
			log.Println("Worker stopping")
			return
		}
	}
}

func main() {
	dbPath := flag.String("db", "data.db", "path to SQLite database")
	pollInterval := flag.Duration("poll", 100*time.Millisecond, "polling interval")
	flag.Parse()
	db, err := InitWorkerDatabase(*dbPath)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer db.Close()

	RunWorker(db, *pollInterval, nil)
}
