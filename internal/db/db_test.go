package db

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestEnqueueFetchMark(t *testing.T) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	defer dbConn.Close()
	// migrate
	if err := InitDB(dbConn); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	// enqueue
	tr := Trade{Account: "acc1", Symbol: "ABCDEF", Volume: 1.0, Open: 1.0, Close: 2.0, Side: "buy"}
	if err := EnqueueTrade(dbConn, tr); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	// fetch
	trs, err := FetchPendingTrades(dbConn)
	if err != nil {
		t.Fatalf("fetch pending failed: %v", err)
	}
	if len(trs) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trs))
	}
	f := trs[0]
	if f.Account != tr.Account || f.Symbol != tr.Symbol {
		t.Errorf("fetched mismatch: got %+v, want %+v", f, tr)
	}
	// mark
	if err := MarkProcessed(dbConn, f.ID); err != nil {
		t.Fatalf("mark processed failed: %v", err)
	}
	// fetch again
	trs2, err := FetchPendingTrades(dbConn)
	if err != nil {
		t.Fatalf("fetch 2 failed: %v", err)
	}
	if len(trs2) != 0 {
		t.Fatalf("expected 0 trades, got %d", len(trs2))
	}
}

func TestUpdateGetStats(t *testing.T) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	defer dbConn.Close()
	// migrate
	if err := InitDB(dbConn); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	// initial stats
	s0, err := GetStats(dbConn, "acc1")
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}
	if s0.Trades != 0 || s0.Profit != 0 {
		t.Errorf("initial stats not zero: %+v", s0)
	}
	// update twice
	if err := UpdateStats(dbConn, "acc1", 100.0); err != nil {
		t.Fatalf("update stats failed: %v", err)
	}
	if err := UpdateStats(dbConn, "acc1", -30.5); err != nil {
		t.Fatalf("update stats 2 failed: %v", err)
	}
	// get final
	s1, err := GetStats(dbConn, "acc1")
	if err != nil {
		t.Fatalf("get stats 2 failed: %v", err)
	}
	if s1.Trades != 2 || s1.Profit != 69.5 {
		t.Errorf("final stats mismatch: %+v", s1)
	}
}
