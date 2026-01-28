package polymarket

import (
	"encoding/json"
	"testing"
)

func TestNormalizeMarketBasic(t *testing.T) {
	raw := RawMarket{
		ID:       "test123",
		Question: strPtr("Will Bitcoin reach $100k?"),
		Outcomes: json.RawMessage(`["Yes", "No"]`),
		OutcomePrices: json.RawMessage(`["0.65", "0.35"]`),
		ClobTokenIds: json.RawMessage(`["tok1", "tok2"]`),
		Active:       boolPtr(true),
		AcceptingOrders: boolPtr(true),
		Slug:         strPtr("bitcoin-100k"),
	}

	m := NormalizeMarket(raw)

	if m.ID != "poly_test123" {
		t.Errorf("ID = %q, want poly_test123", m.ID)
	}
	if m.Question != "Will Bitcoin reach $100k?" {
		t.Errorf("Question = %q", m.Question)
	}
	if len(m.Outcomes) != 2 {
		t.Fatalf("Outcomes len = %d, want 2", len(m.Outcomes))
	}
	if m.Outcomes[0].Name != "Yes" {
		t.Errorf("Outcome[0].Name = %q, want Yes", m.Outcomes[0].Name)
	}
	if m.Outcomes[1].Name != "No" {
		t.Errorf("Outcome[1].Name = %q, want No", m.Outcomes[1].Name)
	}
	if m.Outcomes[0].Price == nil || *m.Outcomes[0].Price != 0.65 {
		t.Errorf("Outcome[0].Price = %v, want 0.65", m.Outcomes[0].Price)
	}
	if !m.IsTradingEnabled {
		t.Error("IsTradingEnabled should be true")
	}
	if m.Source != "Polymarket" {
		t.Errorf("Source = %q, want Polymarket", m.Source)
	}
	if m.SourceUrl != "https://polymarket.com/market/bitcoin-100k" {
		t.Errorf("SourceUrl = %q", m.SourceUrl)
	}
	if len(m.OrderbookTokens) != 2 {
		t.Fatalf("OrderbookTokens len = %d, want 2", len(m.OrderbookTokens))
	}
	if m.OrderbookTokens[0].TokenID != "tok1" {
		t.Errorf("Token[0].TokenID = %q, want tok1", m.OrderbookTokens[0].TokenID)
	}
}

func TestNormalizeMarketStringArray(t *testing.T) {
	// Test with JSON string containing array (Polymarket sometimes sends this)
	raw := RawMarket{
		ID:            "test456",
		Question:      strPtr("Test market"),
		Outcomes:      json.RawMessage(`"[\"Yes\", \"No\"]"`),
		OutcomePrices: json.RawMessage(`"[0.5, 0.5]"`),
		ClobTokenIds:  json.RawMessage(`"[\"a\", \"b\"]"`),
	}

	m := NormalizeMarket(raw)
	if len(m.Outcomes) != 2 {
		t.Fatalf("Outcomes len = %d, want 2", len(m.Outcomes))
	}
}

func TestNormalizeMarketClosedStatus(t *testing.T) {
	raw := RawMarket{
		ID:       "closed1",
		Question: strPtr("Closed market"),
		Closed:   boolPtr(true),
	}

	m := NormalizeMarket(raw)
	if m.Status != "CLOSED" {
		t.Errorf("Status = %q, want CLOSED", m.Status)
	}
}

func TestNormalizeMarketMissingFields(t *testing.T) {
	raw := RawMarket{
		ID: "minimal",
	}

	m := NormalizeMarket(raw)
	if m.ID != "poly_minimal" {
		t.Errorf("ID = %q, want poly_minimal", m.ID)
	}
	if m.Question != "Polymarket Market" {
		t.Errorf("Question = %q, want 'Polymarket Market'", m.Question)
	}
	if m.Status != "OPEN" {
		t.Errorf("Status = %q, want OPEN", m.Status)
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
