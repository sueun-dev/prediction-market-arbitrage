package main

import (
	"math"
	"strings"
	"testing"
)

func TestCalcPerShareEdgeUsesFixedPoint(t *testing.T) {
	gross, net, netBps, ok := calcPerShareEdge(0.50, 0.53, 200, 100)
	if !ok {
		t.Fatal("calcPerShareEdge should succeed")
	}
	if !almostEqual(gross, 0.03, 1e-9) {
		t.Fatalf("gross = %.8f, want 0.03", gross)
	}
	if !almostEqual(net, 0.0147, 1e-9) {
		t.Fatalf("net = %.8f, want 0.0147", net)
	}
	if !almostEqual(netBps, 288.2352941, 1e-6) {
		t.Fatalf("netBps = %.8f, want 288.2352941", netBps)
	}
}

func TestCalcTradeNetUsesFixedPoint(t *testing.T) {
	netUSD, netBps, ok := calcTradeNet(103, 100, 200, 100)
	if !ok {
		t.Fatal("calcTradeNet should succeed")
	}
	if !almostEqual(netUSD, -0.03, 1e-9) {
		t.Fatalf("netUSD = %.8f, want -0.03", netUSD)
	}
	if !almostEqual(netBps, -2.94117647, 1e-6) {
		t.Fatalf("netBps = %.8f, want -2.94117647", netBps)
	}
}

func TestPriceToMicrosRejectsInvalidRange(t *testing.T) {
	cases := []float64{-1, 0, 1.0001, math.NaN(), math.Inf(1)}
	for _, c := range cases {
		if _, ok := priceToMicros(c); ok {
			t.Fatalf("priceToMicros(%v) should be invalid", c)
		}
	}
}

func TestLoadScanConfigFallbackAndClamp(t *testing.T) {
	t.Setenv("ARB_MIN_NET_BPS", "20")
	t.Setenv("ARB_MIN_FILL_RATIO", "1.5")
	cfg := loadScanConfig()
	if !almostEqual(cfg.MinNetBps, 20, 1e-12) {
		t.Fatalf("MinNetBps = %.8f, want 20", cfg.MinNetBps)
	}
	if !almostEqual(cfg.MinFillRatio, 0.99, 1e-12) {
		t.Fatalf("MinFillRatio = %.8f, want 0.99", cfg.MinFillRatio)
	}
}

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

// gateTestConfig is a permissive baseline so each test isolates a single gate.
func gateTestConfig() ScanConfig {
	return ScanConfig{
		MinNetBps:      15,
		MinFillRatio:   0.99,
		MinAbsPrice:    0.02,
		MinNetPerShare: 0.005,
		MaxQuoteSkewMs: 60_000,
	}
}

// healthyOpp is an opportunity that clears every feasibility gate. Individual
// tests then break exactly one property to assert that gate (and only that gate).
func healthyOpp() ArbOpportunity {
	return ArbOpportunity{
		PairID: "p1", Question: "Q", Type: "YES_CROSS",
		BuyPlatform: "Predict", BuyPrice: 0.40,
		SellPlatform: "Polymarket", SellPrice: 0.45,
		NetProfit: 0.03, NetBps: 700,
		BuyQuoteMs: 1_000_000, SellQuoteMs: 1_000_000,
		MaxTradeUSD: 1000,
		Fills:       []SimFill{{SizeUSD: 100, Feasible: true}},
	}
}

func TestFeasibilityGate_HealthyIsExecutable(t *testing.T) {
	o := healthyOpp()
	applyFeasibilityGate(&o, gateTestConfig())
	if !o.Executable {
		t.Fatalf("healthy opp should be executable, got reason %q", o.NotExecReason)
	}
	if !isProfitable(o, gateTestConfig().MinNetBps) {
		t.Fatal("healthy opp above threshold should be profitable")
	}
}

// #1: an opportunity whose buy leg (Polymarket) has no fillable depth must be
// excluded even though its net bps clears the threshold.
func TestFeasibilityGate_ZeroDepthExcluded(t *testing.T) {
	o := healthyOpp()
	// Polymarket book arrives depth-less: no feasible fill, zero max trade.
	o.MaxTradeUSD = 0
	o.Fills = []SimFill{{SizeUSD: 100, Feasible: false}}

	applyFeasibilityGate(&o, gateTestConfig())

	if o.Executable {
		t.Fatal("zero-depth opp must not be executable")
	}
	if isProfitable(o, gateTestConfig().MinNetBps) {
		t.Fatal("zero-depth opp must not be profitable despite positive bps")
	}
	if o.NotExecReason == "" {
		t.Fatal("expected a not-executable reason for zero depth")
	}
}

// #1: even with a non-zero MaxTradeUSD, if no simulated fill is feasible the
// opportunity is not tradeable.
func TestFeasibilityGate_NoFeasibleFillExcluded(t *testing.T) {
	o := healthyOpp()
	o.MaxTradeUSD = 500
	o.Fills = []SimFill{{SizeUSD: 100, Feasible: false}, {SizeUSD: 500, Feasible: false}}

	applyFeasibilityGate(&o, gateTestConfig())

	if o.Executable {
		t.Fatalf("opp with no feasible fill must not be executable")
	}
}

// #2: legs quoted far apart in time must be rejected as stale.
func TestFeasibilityGate_StaleExcluded(t *testing.T) {
	o := healthyOpp()
	// ~118 minutes apart (the audit's observed gap) >> 60s threshold.
	o.SellQuoteMs = o.BuyQuoteMs + 118*60*1000

	applyFeasibilityGate(&o, gateTestConfig())

	if o.Executable {
		t.Fatal("stale opp (legs hours apart) must not be executable")
	}
	if o.QuoteSkewMs != 118*60*1000 {
		t.Fatalf("QuoteSkewMs = %d, want %d", o.QuoteSkewMs, 118*60*1000)
	}
	if !strings.Contains(o.NotExecReason, "stale") {
		t.Fatalf("reason = %q, want it to mention staleness", o.NotExecReason)
	}
}

// #2: a missing timestamp on either leg means freshness can't be verified.
func TestFeasibilityGate_MissingTimestampExcluded(t *testing.T) {
	o := healthyOpp()
	o.SellQuoteMs = 0 // absent

	applyFeasibilityGate(&o, gateTestConfig())

	if o.Executable {
		t.Fatal("opp with a missing leg timestamp must not be executable")
	}
	if o.QuoteSkewMs != -1 {
		t.Fatalf("QuoteSkewMs = %d, want -1 (unknown)", o.QuoteSkewMs)
	}
}

// #3: sub-cent longshots produce enormous bps but must be filtered by the
// absolute-price floor.
func TestFeasibilityGate_LongshotPriceFiltered(t *testing.T) {
	o := healthyOpp()
	// 0.001 -> 0.003 longshot: huge bps, tiny absolute edge, below price floor.
	o.BuyPrice = 0.001
	o.SellPrice = 0.003
	o.NetProfit = 0.0019
	o.NetBps = 19108

	applyFeasibilityGate(&o, gateTestConfig())

	if o.Executable {
		t.Fatal("sub-cent longshot must not be executable")
	}
	if isProfitable(o, gateTestConfig().MinNetBps) {
		t.Fatal("sub-cent longshot must not be profitable despite +19,108 bps")
	}
}

// #3: a tiny absolute net-per-share edge is rejected even at an acceptable price.
func TestFeasibilityGate_TinyNetPerShareFiltered(t *testing.T) {
	o := healthyOpp()
	o.BuyPrice = 0.50   // above price floor
	o.NetProfit = 0.001 // below the $0.005/share floor
	o.NetBps = 20

	applyFeasibilityGate(&o, gateTestConfig())

	if o.Executable {
		t.Fatal("tiny net-per-share opp must not be executable")
	}
}
