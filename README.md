# predict-market-arb

Prediction market arbitrage tooling focused on **Predict.fun** and **Polymarket**. The repository contains:

- Go binaries for fetching market data, building cross-market pairs, serving a dashboard, and scanning arb opportunities
- TypeScript utilities for local data fetching and a lightweight dev server
- Static dashboard assets under `site/`

## Current scope

The live implementation in this repository currently supports:

| Platform | Data source | Status |
| --- | --- | --- |
| Predict.fun | GraphQL + WebSocket orderbook snapshot flow | Implemented |
| Polymarket | Gamma API + CLOB orderbook proxy | Implemented |

Other platforms mentioned in older notes are **not implemented in the current codebase**.

## Repo layout

```text
cmd/
  arb_scan/           # CLI scanner for profitable YES cross opportunities
  fetch_open_markets/ # Fetch Predict.fun markets
  fetch_all_markets/  # Combine Predict.fun + Polymarket and build pairs
  server/             # Go HTTP dashboard server
  ws_debug/           # Debug utility for raw websocket streams

internal/
  config/             # CLI/config parsing helpers
  market/             # Shared market and payload types
  matcher/            # Pairing and text-matching logic
  polymarket/         # Polymarket client + normalization
  predict/            # Predict.fun client + normalization
  server/             # Dashboard HTTP/SSE server
  worker/             # Bounded concurrency helpers

site/
  data/               # Generated JSON payloads
  index.html          # Static dashboard

*.ts                  # TypeScript fetch/server utilities
```

## Arbitrage model

The scanner looks for executable **YES cross** opportunities:

- buy YES on venue A at the best ask
- sell YES on venue B at the best bid
- subtract taker fees
- reject mid-price-only inputs that are not actually executable

This avoids double-counting complement-style math and keeps the output aligned with actionable orderbook prices.

## Quick start

### 1. Build the Go binaries

```bash
make build
```

This produces:

- `bin/fetch_open_markets`
- `bin/fetch_all_markets`
- `bin/server`
- `bin/arb`

### 2. Generate market data

```bash
./bin/fetch_all_markets
```

Quick smoke run with bounded live data:

```bash
POLYMARKET_MAX_MARKETS=1 ./bin/fetch_all_markets \
  --max-markets 1 \
  --skip-orderbook \
  --skip-holders \
  --skip-comments \
  --skip-timeseries
```

Generated files land under `site/data/`:

- `predict_markets_full.json`
- `predict_markets_view.json`
- `markets_full.json`
- `markets_view.json`
- `markets_pairs.json`

### 3. Run the scanner

```bash
./bin/arb
```

Or pass a custom pairs file:

```bash
./bin/arb ./site/data/markets_pairs.json
```

### 4. Run the Go dashboard server

```bash
./bin/server
```

Default server URL:

```text
http://localhost:8050
```

## TypeScript utilities

The TypeScript files are useful for local development and debugging.

Install dependencies:

```bash
npm ci
```

Run the dev server:

```bash
npm run dev
```

Compile TypeScript output:

```bash
npm run build
```

Run type-only verification:

```bash
npm run typecheck
```

Run the TypeScript normalization tests:

```bash
npm run test:ts
```

`npm run build` emits compiled files into `dist/`, which is intentionally ignored by Git.

## Verification

Go tests:

```bash
go test ./...
```

Go build:

```bash
go build ./...
```

TypeScript check:

```bash
npm ci
npm run check
```

Unified local check:

```bash
make check
```

## Useful environment variables

Scanner:

- `ARB_MIN_NET_BPS`
- `ARB_MIN_FILL_RATIO`

Dashboard / refresh loop:

- `PORT`
- `REFRESH_INTERVAL_MS`
- `SSE_PING_MS`
- `AUTO_REFRESH`
- `FETCH_CONCURRENCY`
- `CATEGORY_CONCURRENCY`

Polymarket orderbook proxy:

- `POLYMARKET_CLOB_URL`
- `POLYMARKET_ORDERBOOK_TTL_MS`
- `POLYMARKET_ORDERBOOK_CONCURRENCY`
- `POLYMARKET_ORDERBOOK_LEVELS`
- `POLYMARKET_ORDERBOOK_MAX_TOKENS`
- `POLYMARKET_MAX_MARKETS`

`fetch_all_markets --max-markets N` now limits Predict.fun input and, unless `POLYMARKET_MAX_MARKETS` is explicitly set, applies the same cap to the Polymarket side for smoke tests and small-batch runs.

## Notes

- The Go server reads `site/data/markets_pairs.json` by default.
- `site/data/*.json` is runtime data, not a source of truth.
- If the cached JSON is malformed, the server now rejects it instead of silently serving invalid metadata.
