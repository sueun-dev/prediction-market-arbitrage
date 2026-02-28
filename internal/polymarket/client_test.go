package polymarket

import (
	"encoding/json"
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
