package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	dbm "gitlab.com/digineat/go-broker-test/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	err = dbm.InitDB(db)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	return db
}

func TestInitDatabase(t *testing.T) {
	db, err := InitDatabase(":memory:")
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer db.Close()

	_, err = InitDatabase("/invalid/path/to/db")
	if err == nil {
		t.Error("Expected error with invalid db path, got nil")
	}
}

func TestValidateTradeRequest(t *testing.T) {
	tests := []struct {
		name        string
		request     TradeRequest
		expectError bool
	}{
		{
			name: "Valid request",
			request: TradeRequest{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    1.5,
				Close:   2.0,
				Side:    "buy",
			},
			expectError: false,
		},
		{
			name: "Empty account",
			request: TradeRequest{
				Account: "",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    1.5,
				Close:   2.0,
				Side:    "buy",
			},
			expectError: true,
		},
		{
			name: "Invalid symbol",
			request: TradeRequest{
				Account: "acc1",
				Symbol:  "ABC",
				Volume:  1.0,
				Open:    1.5,
				Close:   2.0,
				Side:    "buy",
			},
			expectError: true,
		},
		{
			name: "Zero volume",
			request: TradeRequest{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  0,
				Open:    1.5,
				Close:   2.0,
				Side:    "buy",
			},
			expectError: true,
		},
		{
			name: "Zero open price",
			request: TradeRequest{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    0,
				Close:   2.0,
				Side:    "buy",
			},
			expectError: true,
		},
		{
			name: "Zero close price",
			request: TradeRequest{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    1.5,
				Close:   0,
				Side:    "buy",
			},
			expectError: true,
		},
		{
			name: "Invalid side",
			request: TradeRequest{
				Account: "acc1",
				Symbol:  "ABCDEF",
				Volume:  1.0,
				Open:    1.5,
				Close:   2.0,
				Side:    "invalid",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTradeRequest(tc.request)
			if tc.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestCalculateProfit(t *testing.T) {
	tests := []struct {
		close    float64
		open     float64
		volume   float64
		side     string
		expected float64
	}{
		{2.0, 1.0, 1.0, "buy", 100000},
		{2.0, 1.0, 2.0, "buy", 200000},
		{2.0, 1.0, 1.0, "sell", -100000},
		{1.0, 2.0, 1.0, "buy", -100000},
	}

	for _, tc := range tests {
		result := CalculateProfit(tc.close, tc.open, tc.volume, tc.side)
		if result != tc.expected {
			t.Errorf("CalculateProfit(%f, %f, %f, %s) = %f; want %f",
				tc.close, tc.open, tc.volume, tc.side, result, tc.expected)
		}
	}
}

func TestHandleTradeRequest(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	trade := TradeRequest{
		Account: "acc1",
		Symbol:  "ABCDEF",
		Volume:  1.0,
		Open:    1.0,
		Close:   2.0,
		Side:    "buy",
	}

	body, _ := json.Marshal(trade)
	req := httptest.NewRequest("POST", "/trades", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleTradeRequest(w, req, db)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status Accepted; got %v", w.Code)
	}

	req = httptest.NewRequest("GET", "/trades", nil)
	w = httptest.NewRecorder()

	HandleTradeRequest(w, req, db)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status MethodNotAllowed; got %v", w.Code)
	}

	req = httptest.NewRequest("POST", "/trades", strings.NewReader("{invalid json}"))
	w = httptest.NewRecorder()

	HandleTradeRequest(w, req, db)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest; got %v", w.Code)
	}

	invalidTrade := TradeRequest{Account: "acc1", Symbol: "invalid", Volume: 1.0, Open: 1.0, Close: 2.0, Side: "buy"}
	body, _ = json.Marshal(invalidTrade)
	req = httptest.NewRequest("POST", "/trades", bytes.NewReader(body))
	w = httptest.NewRecorder()

	HandleTradeRequest(w, req, db)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest; got %v", w.Code)
	}
}

func TestHandleStatsRequest(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dbm.EnqueueTrade(db, dbm.Trade{Account: "testacc", Symbol: "ABCDEF", Volume: 1.0, Open: 1.0, Close: 2.0, Side: "buy"})
	dbm.UpdateStats(db, "testacc", 100000)

	req := httptest.NewRequest("GET", "/stats/testacc", nil)
	w := httptest.NewRecorder()

	HandleStatsRequest(w, req, db)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK; got %v", w.Code)
	}

	var stats dbm.Stats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.Account != "testacc" || stats.Trades < 1 || stats.Profit < 1 {
		t.Errorf("Expected stats with valid data, got %+v", stats)
	}

	req = httptest.NewRequest("POST", "/stats/testacc", nil)
	w = httptest.NewRecorder()

	HandleStatsRequest(w, req, db)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status MethodNotAllowed; got %v", w.Code)
	}

	req = httptest.NewRequest("GET", "/stats/", nil)
	w = httptest.NewRecorder()

	HandleStatsRequest(w, req, db)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest; got %v", w.Code)
	}
}

func TestHandleHealthz(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	HandleHealthz(w, req, db)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK; got %v", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", string(body))
	}

	req = httptest.NewRequest("POST", "/healthz", nil)
	w = httptest.NewRecorder()

	HandleHealthz(w, req, db)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status MethodNotAllowed; got %v", w.Code)
	}
}

func TestSetupRouter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	router := SetupRouter(db)
	if router == nil {
		t.Fatal("Expected router to be created")
	}

	srv := httptest.NewServer(router)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", res.StatusCode)
	}
}

func TestOriginalHealthz(t *testing.T) {
	dbConn, _ := sql.Open("sqlite3", ":memory:")
	defer dbConn.Close()
	dbm.InitDB(dbConn)

	srv := httptest.NewServer(SetupRouter(dbConn))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "OK" {
		t.Errorf("healthz body = %s", body)
	}
}

func TestEndToEndTradeFlow(t *testing.T) {
	dbConn, _ := sql.Open("sqlite3", ":memory:")
	defer dbConn.Close()
	dbm.InitDB(dbConn)

	srv := httptest.NewServer(SetupRouter(dbConn))
	defer srv.Close()

	res, _ := http.Get(srv.URL + "/stats/acc1")
	var s dbm.Stats
	json.NewDecoder(res.Body).Decode(&s)
	if s.Trades != 0 || s.Profit != 0 {
		t.Errorf("initial stats = %+v", s)
	}

	reqBody := map[string]interface{}{"account": "acc1", "symbol": "ABCDEF", "volume": 1.0, "open": 1.0, "close": 2.0, "side": "buy"}
	b, _ := json.Marshal(reqBody)
	res2, err := http.Post(srv.URL+"/trades", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if res2.StatusCode != http.StatusAccepted {
		t.Errorf("POST trades status = %d", res2.StatusCode)
	}

	res3, _ := http.Get(srv.URL + "/stats/acc1")
	json.NewDecoder(res3.Body).Decode(&s)
	if s.Trades != 0 || s.Profit != 0 {
		t.Errorf("after stats = %+v", s)
	}
}
