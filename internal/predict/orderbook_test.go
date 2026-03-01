package predict

import (
	"math"
	"testing"
)

func TestNormalizeOrderbookFiltersAndSortsRows(t *testing.T) {
	snapshot := &OrderbookSnapshot{
		UpdateTimestampMs: 123,
		OrderCount:        7,
		Asks: [][2]float64{
			{0.55, 10},
			{0, 5},
			{math.NaN(), 3},
			{0.45, 0},
			{0.35, 2},
			{0.35, 4},
			{1.2, 1},
		},
		Bids: [][2]float64{
			{0.20, 5},
			{0.40, 0},
			{0.52, 1},
			{0.52, 3},
			{-0.1, 2},
		},
	}

	view := NormalizeOrderbook(snapshot)
	if view == nil {
		t.Fatal("NormalizeOrderbook should return non-nil view")
	}
	if len(view.Asks) != 3 || len(view.Bids) != 3 {
		t.Fatalf("unexpected depth after filtering: asks=%d bids=%d", len(view.Asks), len(view.Bids))
	}
	if !almostEqual(view.Asks[0].Price, 0.35) || !almostEqual(view.Asks[1].Price, 0.35) || !almostEqual(view.Asks[2].Price, 0.55) {
		t.Fatalf("asks are not sorted: %+v", view.Asks)
	}
	if !almostEqual(view.Asks[0].Size, 4) {
		t.Fatalf("ask tie-break by size failed: %+v", view.Asks)
	}
	if !almostEqual(view.Bids[0].Price, 0.52) || !almostEqual(view.Bids[1].Price, 0.52) || !almostEqual(view.Bids[2].Price, 0.20) {
		t.Fatalf("bids are not sorted: %+v", view.Bids)
	}
	if !almostEqual(view.Bids[0].Size, 3) {
		t.Fatalf("bid tie-break by size failed: %+v", view.Bids)
	}
	if view.BestAsk == nil || !almostEqual(*view.BestAsk, 0.35) {
		t.Fatalf("bestAsk incorrect: %+v", view.BestAsk)
	}
	if view.BestBid == nil || !almostEqual(*view.BestBid, 0.52) {
		t.Fatalf("bestBid incorrect: %+v", view.BestBid)
	}
}

func TestNormalizeOrderbookNilSnapshot(t *testing.T) {
	if got := NormalizeOrderbook(nil); got != nil {
		t.Fatalf("NormalizeOrderbook(nil) = %+v, want nil", got)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-12
}
