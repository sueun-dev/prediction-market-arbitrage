package polymarket

import (
	"encoding/json"
	"math"
	"testing"
)

func TestNormalizeBookParsesStringTimestamp(t *testing.T) {
	raw := `{
		"timestamp":"1772288936414",
		"asks":[{"price":"0.51","size":"100"}],
		"bids":[{"price":"0.49","size":"120"}]
	}`

	var payload PolyBookPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	book := NormalizeBook(&payload, 8)
	if book.UpdateTimestampMs == nil {
		t.Fatal("UpdateTimestampMs should not be nil")
	}
	if got, want := *book.UpdateTimestampMs, int64(1772288936414); got != want {
		t.Fatalf("UpdateTimestampMs = %d, want %d", got, want)
	}
	if len(book.Asks) != 1 || len(book.Bids) != 1 {
		t.Fatalf("unexpected book depth: asks=%d bids=%d", len(book.Asks), len(book.Bids))
	}
}

func TestNormalizeBookParsesNumericTimestamp(t *testing.T) {
	raw := `{
		"timestamp":1772288936414,
		"asks":[{"price":"0.60","size":"10"}],
		"bids":[{"price":"0.40","size":"20"}]
	}`

	var payload PolyBookPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	book := NormalizeBook(&payload, 8)
	if book.UpdateTimestampMs == nil {
		t.Fatal("UpdateTimestampMs should not be nil")
	}
	if got, want := *book.UpdateTimestampMs, int64(1772288936414); got != want {
		t.Fatalf("UpdateTimestampMs = %d, want %d", got, want)
	}
}

func TestNormalizeBookInvalidTimestampFallsBackToNil(t *testing.T) {
	raw := `{
		"timestamp":"not-a-number",
		"asks":[{"price":"0.60","size":"10"}],
		"bids":[{"price":"0.40","size":"20"}]
	}`

	var payload PolyBookPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	book := NormalizeBook(&payload, 8)
	if book.UpdateTimestampMs != nil {
		t.Fatalf("UpdateTimestampMs should be nil, got %d", *book.UpdateTimestampMs)
	}
}

func TestNormalizeBookFiltersSortsAndLimits(t *testing.T) {
	raw := `{
		"timestamp":"1772288936414",
		"asks":[
			{"price":"0.55","size":"10"},
			{"price":"bad","size":"3"},
			{"price":"0.45","size":"0"},
			{"price":"0.45","size":"9"},
			{"price":"1.20","size":"2"},
			{"price":"0.45","size":"8"},
			{"price":"0.30","size":"4"}
		],
		"bids":[
			{"price":"0.40","size":"5"},
			{"price":"0.52","size":"2"},
			{"price":"0.52","size":"4"},
			{"price":"0.00","size":"1"},
			{"price":"0.35","size":"-1"}
		]
	}`

	var payload PolyBookPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	book := NormalizeBook(&payload, 2)
	if len(book.Asks) != 2 || len(book.Bids) != 2 {
		t.Fatalf("unexpected book depth: asks=%d bids=%d", len(book.Asks), len(book.Bids))
	}
	if !almostEqual(book.Asks[0].Price, 0.30) || !almostEqual(book.Asks[1].Price, 0.45) {
		t.Fatalf("asks are not sorted/filtered: %+v", book.Asks)
	}
	if !almostEqual(book.Bids[0].Price, 0.52) || !almostEqual(book.Bids[1].Price, 0.52) {
		t.Fatalf("bids are not sorted/filtered: %+v", book.Bids)
	}
	if !almostEqual(book.Bids[0].Size, 4) {
		t.Fatalf("bids tie-break by size failed: %+v", book.Bids)
	}
}

func TestNormalizeBookLevelsZeroKeepsAllValidRows(t *testing.T) {
	raw := `{
		"timestamp":"1772288936414",
		"asks":[
			{"price":"0.51","size":"100"},
			{"price":"0.52","size":"100"},
			{"price":"0.53","size":"100"}
		],
		"bids":[
			{"price":"0.49","size":"100"},
			{"price":"0.48","size":"100"},
			{"price":"0.47","size":"100"}
		]
	}`

	var payload PolyBookPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	book := NormalizeBook(&payload, 0)
	if len(book.Asks) != 3 || len(book.Bids) != 3 {
		t.Fatalf("levels=0 should keep all rows: asks=%d bids=%d", len(book.Asks), len(book.Bids))
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-12
}
