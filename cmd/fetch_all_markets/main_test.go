package main

import (
	"os"
	"reflect"
	"testing"
)

func TestFilterPredictArgsRemovesManagedOutputFlags(t *testing.T) {
	args := []string{
		"--max-markets", "2",
		"--full-out", "/tmp/full.json",
		"--out=/tmp/view.json",
		"--skip-orderbook",
		"--raw-out", "/tmp/raw.json",
	}

	got := filterPredictArgs(args)
	want := []string{
		"--max-markets", "2",
		"--skip-orderbook",
		"--raw-out", "/tmp/raw.json",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterPredictArgs() = %v, want %v", got, want)
	}
}

func TestResolvePolymarketMaxMarketsUsesCLIValueByDefault(t *testing.T) {
	t.Setenv("POLYMARKET_MAX_MARKETS", "")

	got := resolvePolymarketMaxMarkets([]string{"--max-markets", "7"})
	if got != 7 {
		t.Fatalf("resolvePolymarketMaxMarkets() = %d, want 7", got)
	}
}

func TestResolvePolymarketMaxMarketsPrefersEnvOverride(t *testing.T) {
	t.Setenv("POLYMARKET_MAX_MARKETS", "11")

	got := resolvePolymarketMaxMarkets([]string{"--max-markets", "7"})
	if got != 11 {
		t.Fatalf("resolvePolymarketMaxMarkets() = %d, want 11", got)
	}
}

func TestResolvePolymarketMaxMarketsFallsBackToZero(t *testing.T) {
	t.Setenv("POLYMARKET_MAX_MARKETS", "")
	_ = os.Unsetenv("POLYMARKET_MAX_MARKETS")

	got := resolvePolymarketMaxMarkets(nil)
	if got != 0 {
		t.Fatalf("resolvePolymarketMaxMarkets() = %d, want 0", got)
	}
}
