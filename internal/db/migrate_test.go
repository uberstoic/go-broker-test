package db

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestInitDB(t *testing.T) {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer conn.Close()

	if err := InitDB(conn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	expected := []string{"trades_q", "account_stats"}
	for _, tbl := range expected {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name = ?"
		err := conn.QueryRow(query, tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", tbl, err)
		}
		if name != tbl {
			t.Errorf("expected table name %s, got %s", tbl, name)
		}
	}
}
