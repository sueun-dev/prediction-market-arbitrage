package main

import (
	"math"
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
