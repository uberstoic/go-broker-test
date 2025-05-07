package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	dbm "gitlab.com/digineat/go-broker-test/internal/db"
)

var symbolRe = regexp.MustCompile(`^[A-Z]{6}$`)

type TradeRequest struct {
	Account string  `json:"account"`
	Symbol  string  `json:"symbol"`
	Volume  float64 `json:"volume"`
	Open    float64 `json:"open"`
	Close   float64 `json:"close"`
	Side    string  `json:"side"`
}

func InitDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
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

func ValidateTradeRequest(req TradeRequest) error {
	if req.Account == "" || !symbolRe.MatchString(req.Symbol) || req.Volume <= 0 || req.Open <= 0 || req.Close <= 0 || (req.Side != "buy" && req.Side != "sell") {
		return fmt.Errorf("invalid trade payload")
	}
	return nil
}

func CalculateProfit(close, open, volume float64, side string) float64 {
	profit := (close - open) * volume * 100000.0
	if side == "sell" {
		profit = -profit
	}
	return profit
}

func HandleTradeRequest(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if err := ValidateTradeRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := dbm.EnqueueTrade(db, dbm.Trade{
		Account: req.Account,
		Symbol:  req.Symbol,
		Volume:  req.Volume,
		Open:    req.Open,
		Close:   req.Close,
		Side:    req.Side,
	}); err != nil {
		http.Error(w, "failed to enqueue trade", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func HandleStatsRequest(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	acc := strings.TrimPrefix(r.URL.Path, "/stats/")
	if acc == "" {
		http.Error(w, "account not specified", http.StatusBadRequest)
		return
	}

	s, err := dbm.GetStats(db, acc)
	if err != nil {
		http.Error(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	s.Account = acc
	s.Profit = math.Round(s.Profit*100) / 100
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"Account":"%s","Trades":%d,"Profit":%.2f}`, s.Account, s.Trades, s.Profit)
}

func HandleHealthz(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := db.Ping(); err != nil {
		http.Error(w, "db error", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func SetupRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// POST /trades endpoint
	mux.HandleFunc("/trades", func(w http.ResponseWriter, r *http.Request) {
		HandleTradeRequest(w, r, db)
	})

	// GET /stats/{acc} endpoint
	mux.HandleFunc("/stats/", func(w http.ResponseWriter, r *http.Request) {
		HandleStatsRequest(w, r, db)
	})

	// GET /healthz endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		HandleHealthz(w, r, db)
	})

	return mux
}

func main() {
	// Command line flags
	dbPath := flag.String("db", "data.db", "path to SQLite database")
	listenAddr := flag.String("listen", "8080", "HTTP server listen address")
	flag.Parse()

	// Initialize database connection
	db, err := InitDatabase(*dbPath)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer db.Close()

	// Set up router with handlers
	mux := SetupRouter(db)

	// Start server
	serverAddr := fmt.Sprintf(":%s", *listenAddr)
	log.Printf("Starting server on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
