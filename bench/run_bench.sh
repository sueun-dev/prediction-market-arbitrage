#!/usr/bin/env bash
set -euo pipefail

BENCH_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$BENCH_DIR")"
ROUNDS=3

echo "============================================"
echo "  Predict Market Fetch Benchmark"
echo "  Rounds per language: $ROUNDS"
echo "  $(date)"
echo "============================================"
echo ""

run_bench() {
  local label="$1"
  shift
  echo "--- $label ---"
  for i in $(seq 1 "$ROUNDS"); do
    echo -n "  Run $i: "
    "$@" 2>/dev/null
    sleep 1
  done
  echo ""
}

run_bench "TypeScript (Node.js)" npx tsx "$BENCH_DIR/bench.ts"
run_bench "Python 3" python3 "$BENCH_DIR/bench.py"
run_bench "C (libcurl)" "$BENCH_DIR/bench_c"
run_bench "C++ (libcurl)" "$BENCH_DIR/bench_cpp"
run_bench "Go" "$BENCH_DIR/bench_go"
run_bench "Rust (reqwest blocking)" "$BENCH_DIR/bench_rust/target/release/bench_rust"

echo "============================================"
echo "  Benchmark complete"
echo "============================================"
