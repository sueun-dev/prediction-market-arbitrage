package predict

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gorilla/websocket"

	"predict-market/internal/market"
)

// FetchOrderbookSnapshot connects to the Predict.Fun WebSocket and waits
// for a single orderbook snapshot for the given market ID.
func FetchOrderbookSnapshot(ctx context.Context, marketID string, timeoutMs int) (*OrderbookSnapshot, error) {
	topic := fmt.Sprintf("predictOrderbook/%s", marketID)
	timeout := time.Duration(timeoutMs) * time.Millisecond

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{},
	}
	conn, _, err := dialer.Dial(WsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ws dial: %w", err)
	}
	defer conn.Close()

	// Subscribe
	sub := WsSubscribe{
		RequestID: 1,
		Method:    "subscribe",
		Params:    []string{topic},
	}
	if err := conn.WriteJSON(sub); err != nil {
		return nil, fmt.Errorf("ws subscribe: %w", err)
	}

	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("ws read %s: %w", marketID, err)
		}

		var msg WsMessage
		if json.Unmarshal(msgBytes, &msg) != nil {
			continue
		}

		if msg.Type == "M" && msg.Topic == topic && msg.Data != nil {
			return msg.Data, nil
		}
	}
}

// NormalizeOrderbook converts a raw snapshot to an OrderbookView.
func NormalizeOrderbook(snapshot *OrderbookSnapshot) *market.OrderbookView {
	if snapshot == nil {
		return nil
	}

	normalizeRows := func(rows [][2]float64, isAsk bool) []market.OrderbookRow {
		valid := make([]market.OrderbookRow, 0, len(rows))
		for _, row := range rows {
			p, s := row[0], row[1]
			if math.IsNaN(p) || math.IsNaN(s) || math.IsInf(p, 0) || math.IsInf(s, 0) {
				continue
			}
			if p <= 0 || p > 1 || s <= 0 {
				continue
			}
			valid = append(valid, market.OrderbookRow{Price: p, Size: s})
		}
		sort.Slice(valid, func(i, j int) bool {
			if valid[i].Price == valid[j].Price {
				return valid[i].Size > valid[j].Size
			}
			if isAsk {
				return valid[i].Price < valid[j].Price
			}
			return valid[i].Price > valid[j].Price
		})
		return valid
	}

	asks := normalizeRows(snapshot.Asks, true)
	bids := normalizeRows(snapshot.Bids, false)

	var bestAsk, bestBid, spread, spreadCents *float64
	if len(asks) > 0 {
		bestAsk = &asks[0].Price
	}
	if len(bids) > 0 {
		bestBid = &bids[0].Price
	}
	if bestAsk != nil && bestBid != nil {
		s := roundTo(*bestAsk-*bestBid, 6)
		spread = &s
		sc := roundTo(s*100, 4)
		spreadCents = &sc
	}

	var lastOrder *market.LastOrderSettled
	if snapshot.LastOrderSettled != nil {
		lastOrder = &market.LastOrderSettled{
			ID:       snapshot.LastOrderSettled.ID,
			Price:    snapshot.LastOrderSettled.Price,
			Kind:     snapshot.LastOrderSettled.Kind,
			Side:     snapshot.LastOrderSettled.Side,
			Outcome:  snapshot.LastOrderSettled.Outcome,
			MarketID: snapshot.LastOrderSettled.MarketID,
		}
	}

	ts := snapshot.UpdateTimestampMs

	return &market.OrderbookView{
		UpdateTimestampMs:  &ts,
		OrderCount:         &snapshot.OrderCount,
		LastOrderSettled:   lastOrder,
		BestAsk:            bestAsk,
		BestBid:            bestBid,
		Spread:             spread,
		SpreadCents:        spreadCents,
		Asks:               asks,
		Bids:               bids,
		SettlementsPending: snapshot.SettlementsPending,
	}
}

func roundTo(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}
