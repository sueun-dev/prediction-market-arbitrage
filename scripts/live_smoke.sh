#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/predict-market-live-smoke.XXXXXX")"
GO_PORT="${GO_PORT:-18150}"
TS_PORT="${TS_PORT:-18151}"
SMOKE_MAX_MARKETS="${SMOKE_MAX_MARKETS:-3}"
GO_PID=""
TS_PID=""
PAIRS_BACKUP="$TMP_DIR/markets_pairs.original.json"

cleanup() {
  if [[ -n "$GO_PID" ]] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [[ -n "$TS_PID" ]] && kill -0 "$TS_PID" 2>/dev/null; then
    kill "$TS_PID" 2>/dev/null || true
    wait "$TS_PID" 2>/dev/null || true
  fi
  if [[ -f "$PAIRS_BACKUP" ]]; then
    cp "$PAIRS_BACKUP" "$ROOT/site/data/markets_pairs.json"
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

cd "$ROOT"

if [[ -f site/data/markets_pairs.json ]]; then
  cp site/data/markets_pairs.json "$PAIRS_BACKUP"
fi

echo "[1/6] Building and running automated checks"
make build
npm run check >/dev/null

echo "[2/6] Running bounded live fetch_open_markets"
./bin/fetch_open_markets \
  --max-markets "$SMOKE_MAX_MARKETS" \
  --skip-orderbook \
  --skip-holders \
  --skip-comments \
  --skip-timeseries \
  --full-out "$TMP_DIR/predict_full.json" \
  --out "$TMP_DIR/predict_view.json" \
  --raw-out "$TMP_DIR/predict_raw.json" >/dev/null

TMP_DIR_ENV="$TMP_DIR" python3 - <<'PY'
import json
import os
from pathlib import Path
base = Path(os.environ["TMP_DIR_ENV"])
for name in ("predict_full.json", "predict_view.json", "predict_raw.json"):
    data = json.loads((base / name).read_text())
    assert data["count"] > 0, f"{name} should contain markets"
PY

echo "[3/6] Running bounded live fetch_all_markets"
POLYMARKET_MAX_MARKETS="$SMOKE_MAX_MARKETS" \
./bin/fetch_all_markets \
  --max-markets "$SMOKE_MAX_MARKETS" \
  --skip-orderbook \
  --skip-holders \
  --skip-comments \
  --skip-timeseries >/dev/null

ROOT_ENV="$ROOT" python3 - <<'PY'
import json
import os
from pathlib import Path
data = json.loads(
    Path(os.environ["ROOT_ENV"]).joinpath("site/data/markets_pairs.json").read_text()
)
assert isinstance(data.get("pairs"), list), "pairs must be a JSON array"
assert data.get("count") == len(data.get("pairs", [])), "count must match pairs length"
PY

echo "[4/6] Starting Go server smoke"
PORT="$GO_PORT" AUTO_REFRESH=0 MAX_MARKETS="$SMOKE_MAX_MARKETS" POLYMARKET_MAX_MARKETS="$SMOKE_MAX_MARKETS" \
SKIP_ORDERBOOK=1 SKIP_HOLDERS=1 SKIP_COMMENTS=1 SKIP_TIMESERIES=1 \
./bin/server >"$TMP_DIR/go_server.log" 2>&1 &
GO_PID=$!
sleep 2

GO_PORT_ENV="$GO_PORT" python3 - <<'PY'
import json
import os
import urllib.request
base = "http://127.0.0.1:" + os.environ["GO_PORT_ENV"]
status = json.load(urllib.request.urlopen(base + "/api/status"))
markets = json.load(urllib.request.urlopen(base + "/api/markets"))
assert status["updating"] is False
assert isinstance(markets.get("pairs"), list), "Go /api/markets pairs must be array"
PY

echo "[5/6] Starting TypeScript server smoke"
PORT="$TS_PORT" AUTO_REFRESH=0 MAX_MARKETS="$SMOKE_MAX_MARKETS" POLYMARKET_MAX_MARKETS="$SMOKE_MAX_MARKETS" \
SKIP_ORDERBOOK=1 SKIP_HOLDERS=1 SKIP_COMMENTS=1 SKIP_TIMESERIES=1 \
npm run dev >"$TMP_DIR/ts_server.log" 2>&1 &
TS_PID=$!
sleep 2

TS_PORT_ENV="$TS_PORT" python3 - <<'PY'
import json
import os
import urllib.request
base = "http://127.0.0.1:" + os.environ["TS_PORT_ENV"]
status = json.load(urllib.request.urlopen(base + "/api/status"))
markets = json.load(urllib.request.urlopen(base + "/api/markets"))
assert status["updating"] is False
assert isinstance(markets.get("pairs"), list), "TS /api/markets pairs must be array"
PY

echo "[6/6] Comparing live smoke output shapes"
GO_PORT_ENV="$GO_PORT" TS_PORT_ENV="$TS_PORT" python3 - <<'PY'
import json
import os
import urllib.request
go_data = json.load(
    urllib.request.urlopen(
        "http://127.0.0.1:" + os.environ["GO_PORT_ENV"] + "/api/markets"
    )
)
ts_data = json.load(
    urllib.request.urlopen(
        "http://127.0.0.1:" + os.environ["TS_PORT_ENV"] + "/api/markets"
    )
)
assert go_data["count"] == ts_data["count"], "Go/TS market counts diverged"
assert len(go_data.get("pairs", [])) == len(ts_data.get("pairs", [])), "Go/TS pair lengths diverged"
PY

echo "live smoke passed"
