#!/usr/bin/env bash
set -euo pipefail

BENCH_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$BENCH_DIR")"
ROUNDS=5

echo "============================================"
echo "  HFT Benchmark - Per-Request Latency"
echo "  Rounds: $ROUNDS | $(date)"
echo "============================================"
echo ""

echo "=== Binary Sizes ==="
printf "  %-20s %s\n" "C (libcurl)" "$(du -h "$BENCH_DIR/hft_bench_c" | awk '{print $1}')"
printf "  %-20s %s\n" "C++ (libcurl)" "$(du -h "$BENCH_DIR/hft_bench_cpp" | awk '{print $1}')"
printf "  %-20s %s\n" "Go" "$(du -h "$BENCH_DIR/hft_bench_go" | awk '{print $1}')"
printf "  %-20s %s\n" "Rust" "$(du -h "$BENCH_DIR/bench_rust_hft/target/release/bench_rust_hft" | awk '{print $1}')"
printf "  %-20s %s\n" "TypeScript" "(interpreted)"
printf "  %-20s %s\n" "Python" "(interpreted)"
echo ""

run_hft() {
  local label="$1"
  shift
  echo "--- $label ---"
  for i in $(seq 1 "$ROUNDS"); do
    echo -n "  [$i] "
    "$@" 2>/dev/null
    sleep 2
  done
  echo ""
}

run_hft "C (libcurl, keepalive)" "$BENCH_DIR/hft_bench_c"
run_hft "C++ (libcurl, keepalive)" "$BENCH_DIR/hft_bench_cpp"
run_hft "Go (net/http, pool)" "$BENCH_DIR/hft_bench_go"
run_hft "Rust (reqwest, pool)" "$BENCH_DIR/bench_rust_hft/target/release/bench_rust_hft"
run_hft "TypeScript (Node.js fetch)" npx tsx "$BENCH_DIR/hft_bench.ts"
run_hft "Python 3 (urllib)" python3 "$BENCH_DIR/hft_bench.py"

echo "============================================"
echo "  HFT Benchmark complete"
echo "============================================"
