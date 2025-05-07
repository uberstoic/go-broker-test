# Technical Assignment

The candidate is asked to build a small project consisting of two Go processes:

- The **API server** accepts trade submissions via POST `/trades` and writes them to a queue table.
- The **worker process** continuously reads new trades, validates them, stores them, and instantly updates the account's profit.

Your code must compile, pass `go vet` and `go test -race`, and run via `docker-compose up`.

## Purpose of This Task

We want to assess your ability to write clean HTTP code, work with SQL, and reason about concurrency.

## What You Should Build

### Components and Architecture

```text
┌──────────────┐ POST /trades       ┌───────────────┐
│  API Server  │ ─────────────────► │  Queue Table  │
│  cmd/server  │                    └───────────────┘
└──────────────┘                         ▲
                                         │ SELECT … FOR UPDATE
┌──────────────┐            UPDATE stats │
│  Worker      │ ◄───────────────────────┘
│  cmd/worker  │
└──────────────┘
```

- **API (HTTP)** — exposes one POST endpoint and one GET endpoint.
- **Queue** — an SQLite table `trades_q` used by the API to enqueue trades, and by the worker to mark them as processed.
- **Worker** — a separate process that polls the queue every 100ms, calculates `profit`, and updates `account_stats`.

### Trade Input Format

| Field     | Type    | Validation Rule            |
| -         | -       | -                          |
| `account` | string  | must not be empty          |
| `symbol`  | string  | `^[A-Z]{6}$` (e.g. EURUSD) |
| `volume`  | float64 | must be > 0                |
| `open`    | float64 | must be > 0                |
| `close`   | float64 | must be > 0                |
| `side`    | string  | either "buy" or "sell"     |

Profit calculation (performed by the worker):

```go
lot := 100000.0
profit := (close - open) * volume * lot
if side == "sell" { profit = -profit }
```

### HTTP Contracts

| Method | URL            | Request / Response                               | Expected Behavior                                     |
| -      | -              | -                                                | -                                                     |
| POST   | `/trades`      | JSON trade payload                               | Enqueue trade; respond with 200 OK or 400 on errors   |
| GET    | `/stats/{acc}` | `{"account":"123","trades":37,"profit":1234.56}` | Return current statistics for the given account       |
| GET    | `/healthz`     | plain text OK                                    | Health check endpoint (for Kubernetes liveness probe) |

### How to Run

```shell
make vet
make test
make docker-up

# Sample requests (localhost:8080):
curl -X POST http://localhost:8080/trades \
     -H 'Content-Type: application/json' \
     -d '{"account":"123","symbol":"EURUSD","volume":1.0,"open":1.1000,"close":1.1050,"side":"buy"}'

curl http://localhost:8080/stats/123

make docker-down
```

## What We Expect from Your Code

| Requirement                        | Minimum / Bonus           |
| -                                  | -                         |
| Go 1.24+, only stdlib + light libs | sqlx / chi / validator OK |
| `go vet` and `go test -race` pass  | required                  |
| Test coverage                      | ≥ 60%                     |
| Dockerfile + docker-compose.yml    | bonus (+1)                |
| README: how to run + curl examples | required                  |

## How We Will Evaluate

CI script (GitLab CI) will:

- Run `go vet` and `go test -race -covermode=atomic`.
- Launch both API and worker processes in the background.
- Send one invalid and one valid POST request.
- Fetch and validate the response from `/stats`.
