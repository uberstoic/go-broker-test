package db

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestProcessPending(t *testing.T) {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed open db: %v", err)
	}
	defer conn.Close()
	// migrate schema
	if err := InitDB(conn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	// enqueue trades
	trades := []Trade{
		{Account: "acc1", Symbol: "ABCDEF", Volume: 1.0, Open: 1.0, Close: 2.0, Side: "buy"},
		{Account: "acc1", Symbol: "ABCDEF", Volume: 0.5, Open: 2.0, Close: 1.5, Side: "sell"},
	}
	for _, tr := range trades {
		if err := EnqueueTrade(conn, tr); err != nil {
			t.Fatalf("EnqueueTrade failed: %v", err)
		}
	}
	// process
	if err := ProcessPending(conn); err != nil {
		t.Fatalf("ProcessPending failed: %v", err)
	}
	// check stats
	s, err := GetStats(conn, "acc1")
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	// expected: buy profit = (2-1)*1*100000 = 100000; sell profit = (1.5-2)*0.5*100000 = -25000; total = 125000
	if s.Trades != 2 || s.Profit != 125000 {
		t.Errorf("unexpected stats: %+v", s)
	}
	// check processed flag
	for id := 1; id <= len(trades); id++ {
		var pr int
		row := conn.QueryRow("SELECT processed FROM trades_q WHERE id = ?", id)
		if err := row.Scan(&pr); err != nil {
			t.Errorf("scan processed for id %d failed: %v", id, err)
			continue
		}
		if pr != 1 {
			t.Errorf("trade %d not processed (got %d)", id, pr)
		}
	}
}
