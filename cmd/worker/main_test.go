package main

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	dbm "gitlab.com/digineat/go-broker-test/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	err = dbm.InitDB(db)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db
}

func TestInitWorkerDatabase(t *testing.T) {
	db, err := InitWorkerDatabase(":memory:")
	if err != nil {
		t.Fatalf("InitWorkerDatabase failed: %v", err)
	}
	defer db.Close()

	_, err = InitWorkerDatabase("/invalid/path/to/db")
	if err == nil {
		t.Error("Expected error with invalid db path, got nil")
	}
}

func TestCalculateProfitFromTrade(t *testing.T) {
	tests := []struct {
		name     string
		trade    dbm.Trade
		expected float64
	}{
		{
			name: "Buy position with profit",
			trade: dbm.Trade{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    1.0,
				Close:   2.0,
				Side:    "buy",
			},
			expected: 100000,
		},
		{
			name: "Buy position with loss",
			trade: dbm.Trade{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    2.0,
				Close:   1.0,
				Side:    "buy",
			},
			expected: -100000,
		},
		{
			name: "Sell position with profit",
			trade: dbm.Trade{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    2.0,
				Close:   1.0,
				Side:    "sell",
			},
			expected: 100000,
		},
		{
			name: "Sell position with loss",
			trade: dbm.Trade{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    1.0,
				Close:   2.0,
				Side:    "sell",
			},
			expected: -100000,
		},
		{
			name: "Double volume",
			trade: dbm.Trade{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  2.0,
				Open:    1.0,
				Close:   2.0,
				Side:    "buy",
			},
			expected: 200000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateProfitFromTrade(tt.trade)
			if result != tt.expected {
				t.Errorf("CalculateProfitFromTrade() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProcessTrade(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	trade := dbm.Trade{
		ID:      1,
		Account: "acc1",
		Symbol:  "ABCDEF",
		Volume:  1.0,
		Open:    1.0,
		Close:   2.0,
		Side:    "buy",
	}

	_, err := db.Exec("INSERT INTO trades_q (id, account, symbol, volume, open, close, side, processed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		trade.ID, trade.Account, trade.Symbol, trade.Volume, trade.Open, trade.Close, trade.Side, 0)
	if err != nil {
		t.Fatalf("Failed to insert test trade: %v", err)
	}

	err = ProcessTrade(db, trade)
	if err != nil {
		t.Errorf("ProcessTrade() error = %v", err)
	}
	var processed int
	err = db.QueryRow("SELECT processed FROM trades_q WHERE id = ?", trade.ID).Scan(&processed)
	if err != nil {
		t.Fatalf("Failed to query trade processed status: %v", err)
	}
	if processed != 1 { // 1 means processed
		t.Errorf("Expected trade processed to be 1, got %d", processed)
	}

	var count int
	var profit float64
	err = db.QueryRow("SELECT trades, profit FROM account_stats WHERE account = ?", trade.Account).Scan(&count, &profit)
	if err != nil {
		t.Fatalf("Failed to query account_stats: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected trade count to be 1, got %d", count)
	}
	if profit != 100000 {
		t.Errorf("Expected profit to be 100000, got %f", profit)
	}
}

func TestProcessPendingTrades(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	trades := []dbm.Trade{
		{
			ID:      1,
			Account: "acc1",
			Symbol:  "ABCDEF",
			Volume:  1.0,
			Open:    1.0,
			Close:   2.0,
			Side:    "buy",
		},
		{
			ID:      2,
			Account: "acc2",
			Symbol:  "ABCDEF",
			Volume:  2.0,
			Open:    2.0,
			Close:   3.0,
			Side:    "sell",
		},
	}

	for _, trade := range trades {
		_, err := db.Exec("INSERT INTO trades_q (id, account, symbol, volume, open, close, side, processed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			trade.ID, trade.Account, trade.Symbol, trade.Volume, trade.Open, trade.Close, trade.Side, 0) // 0 for pending/not processed
		if err != nil {
			t.Fatalf("Failed to insert test trade: %v", err)
		}
	}

	count, err := ProcessPendingTrades(db)
	if err != nil {
		t.Errorf("ProcessPendingTrades() error = %v", err)
	}
	if count != 2 {
		t.Errorf("ProcessPendingTrades() = %v, want %v", count, 2)
	}

	var pendingCount int
	err = db.QueryRow("SELECT COUNT(*) FROM trades_q WHERE processed = 0").Scan(&pendingCount)
	if err != nil {
		t.Fatalf("Failed to query pending trades: %v", err)
	}
	if pendingCount != 0 {
		t.Errorf("Expected 0 pending trades, got %d", pendingCount)
	}

	var acc1Profit, acc2Profit float64
	err = db.QueryRow("SELECT profit FROM account_stats WHERE account = 'acc1'").Scan(&acc1Profit)
	if err != nil {
		t.Fatalf("Failed to query acc1 stats: %v", err)
	}
	err = db.QueryRow("SELECT profit FROM account_stats WHERE account = 'acc2'").Scan(&acc2Profit)
	if err != nil {
		t.Fatalf("Failed to query acc2 stats: %v", err)
	}

	if acc1Profit != 100000 {
		t.Errorf("Expected acc1 profit to be 100000, got %f", acc1Profit)
	}
	if acc2Profit != -200000 {
		t.Errorf("Expected acc2 profit to be -200000, got %f", acc2Profit)
	}
}

func TestRunWorker(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec("INSERT INTO trades_q (account, symbol, volume, open, close, side, processed) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"acc1", "ABCDEF", 1.0, 1.0, 2.0, "buy", 0)
	if err != nil {
		t.Fatalf("Failed to insert test trade: %v", err)
	}
	stopCh := make(chan struct{})

	go RunWorker(db, 10*time.Millisecond, stopCh)

	time.Sleep(50 * time.Millisecond)

	close(stopCh)

	var pendingCount int
	err = db.QueryRow("SELECT COUNT(*) FROM trades_q WHERE processed = 0").Scan(&pendingCount)
	if err != nil {
		t.Fatalf("Failed to query pending trades: %v", err)
	}
	if pendingCount != 0 {
		t.Errorf("Expected 0 pending trades, got %d", pendingCount)
	}

	var profit float64
	err = db.QueryRow("SELECT profit FROM account_stats WHERE account = 'acc1'").Scan(&profit)
	if err != nil {
		t.Fatalf("Failed to query stats: %v", err)
	}
	if profit != 100000 {
		t.Errorf("Expected profit to be 100000, got %f", profit)
	}
}
