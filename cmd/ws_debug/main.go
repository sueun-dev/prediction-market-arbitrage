package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// Capture raw WS data from both platforms and print orderbook levels
// to verify bid/ask sorting assumptions.

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: ws_debug <poly|predict> [id]\n")
		fmt.Fprintf(os.Stderr, "  poly <token_id>     — subscribe to Polymarket CLOB token\n")
		fmt.Fprintf(os.Stderr, "  predict <market_id> — subscribe to Predict.Fun market\n")
		os.Exit(1)
	}

	platform := os.Args[1]
	switch platform {
	case "poly":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "need token_id\n")
			os.Exit(1)
		}
		debugPoly(os.Args[2])
	case "predict":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "need market_id\n")
			os.Exit(1)
		}
		debugPredict(os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "unknown platform: %s\n", platform)
		os.Exit(1)
	}
}

func debugPoly(tokenID string) {
	fmt.Printf("=== Polymarket CLOB WS Debug ===\n")
	fmt.Printf("Token: %s\n\n", tokenID)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{},
	}
	conn, _, err := dialer.Dial("wss://ws-subscriptions-clob.polymarket.com/ws/market", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	sub := map[string]interface{}{
		"assets_ids": []string{tokenID},
		"type":       "market",
	}
	if err := conn.WriteJSON(sub); err != nil {
		fmt.Fprintf(os.Stderr, "subscribe error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Subscribed. Waiting for book events...\n\n")

	// PING goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			conn.WriteMessage(websocket.TextMessage, []byte("PING"))
		}
	}()

	bookCount := 0
	for bookCount < 5 {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			break
		}

		if string(msg) == "PONG" {
			continue
		}

		var events []json.RawMessage
		if json.Unmarshal(msg, &events) != nil {
			var single json.RawMessage
			if json.Unmarshal(msg, &single) == nil {
				events = []json.RawMessage{single}
			} else {
				continue
			}
		}

		for _, raw := range events {
			var evt struct {
				EventType string `json:"event_type"`
				AssetID   string `json:"asset_id"`
				Bids      []struct {
					Price string `json:"price"`
					Size  string `json:"size"`
				} `json:"bids"`
				Asks []struct {
					Price string `json:"price"`
					Size  string `json:"size"`
				} `json:"asks"`
			}
			if json.Unmarshal(raw, &evt) != nil {
				continue
			}

			if evt.EventType != "book" {
				continue
			}

			bookCount++
			fmt.Printf("--- Book Event #%d (asset=%s) ---\n", bookCount, evt.AssetID)

			fmt.Printf("BIDS (%d levels):\n", len(evt.Bids))
			for i, b := range evt.Bids {
				tag := ""
				if i == 0 {
					tag = " ← [0] FIRST"
				}
				if i == len(evt.Bids)-1 {
					tag = " ← [last] LAST"
					if i == 0 {
						tag = " ← [0] FIRST & LAST"
					}
				}
				fmt.Printf("  [%d] price=%-10s size=%-12s%s\n", i, b.Price, b.Size, tag)
			}

			fmt.Printf("ASKS (%d levels):\n", len(evt.Asks))
			for i, a := range evt.Asks {
				tag := ""
				if i == 0 {
					tag = " ← [0] FIRST"
				}
				if i == len(evt.Asks)-1 {
					tag = " ← [last] LAST"
					if i == 0 {
						tag = " ← [0] FIRST & LAST"
					}
				}
				fmt.Printf("  [%d] price=%-10s size=%-12s%s\n", i, a.Price, a.Size, tag)
			}

			// Analysis
			if len(evt.Bids) >= 2 {
				p0, _ := strconv.ParseFloat(evt.Bids[0].Price, 64)
				pN, _ := strconv.ParseFloat(evt.Bids[len(evt.Bids)-1].Price, 64)
				if p0 < pN {
					fmt.Printf("  → Bids ASCENDING (first=%.4f < last=%.4f) → best bid = LAST\n", p0, pN)
				} else if p0 > pN {
					fmt.Printf("  → Bids DESCENDING (first=%.4f > last=%.4f) → best bid = FIRST\n", p0, pN)
				} else {
					fmt.Printf("  → Bids SAME price (first=%.4f == last=%.4f)\n", p0, pN)
				}
			}
			if len(evt.Asks) >= 2 {
				p0, _ := strconv.ParseFloat(evt.Asks[0].Price, 64)
				pN, _ := strconv.ParseFloat(evt.Asks[len(evt.Asks)-1].Price, 64)
				if p0 < pN {
					fmt.Printf("  → Asks ASCENDING (first=%.4f < last=%.4f) → best ask = FIRST\n", p0, pN)
				} else if p0 > pN {
					fmt.Printf("  → Asks DESCENDING (first=%.4f > last=%.4f) → best ask = LAST\n", p0, pN)
				} else {
					fmt.Printf("  → Asks SAME price (first=%.4f == last=%.4f)\n", p0, pN)
				}
			}

			if len(evt.Bids) > 0 && len(evt.Asks) > 0 {
				bestBidLast, _ := strconv.ParseFloat(evt.Bids[len(evt.Bids)-1].Price, 64)
				bestAskFirst, _ := strconv.ParseFloat(evt.Asks[0].Price, 64)
				bestBidFirst, _ := strconv.ParseFloat(evt.Bids[0].Price, 64)
				bestAskLast, _ := strconv.ParseFloat(evt.Asks[len(evt.Asks)-1].Price, 64)

				fmt.Printf("\n  Spread check (bid < ask = normal):\n")
				fmt.Printf("    Bids[last]=%.4f  vs Asks[0]=%.4f  → spread=%.4f %s\n",
					bestBidLast, bestAskFirst, bestAskFirst-bestBidLast,
					iff(bestBidLast < bestAskFirst, "✅ NORMAL", "❌ CROSSED"))
				fmt.Printf("    Bids[0]=%.4f    vs Asks[last]=%.4f → spread=%.4f %s\n",
					bestBidFirst, bestAskLast, bestAskLast-bestBidFirst,
					iff(bestBidFirst < bestAskLast, "✅ NORMAL", "❌ CROSSED"))
			}
			fmt.Println()
		}
	}
}

func debugPredict(marketID string) {
	fmt.Printf("=== Predict.Fun WS Debug ===\n")
	fmt.Printf("Market: %s\n\n", marketID)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{},
	}
	conn, _, err := dialer.Dial("wss://ws.predict.fun/ws", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	sub := map[string]interface{}{
		"requestId": 1,
		"method":    "subscribe",
		"params":    []string{"predictOrderbook/" + marketID},
	}
	if err := conn.WriteJSON(sub); err != nil {
		fmt.Fprintf(os.Stderr, "subscribe error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Subscribed to predictOrderbook/%s\n\n", marketID)

	snapCount := 0
	for snapCount < 5 {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			break
		}

		var wsMsg struct {
			Type  string          `json:"type"`
			Topic string          `json:"topic"`
			Data  json.RawMessage `json:"data"`
		}
		if json.Unmarshal(msg, &wsMsg) != nil || wsMsg.Type != "M" {
			continue
		}

		var snap struct {
			Asks [][2]float64 `json:"asks"`
			Bids [][2]float64 `json:"bids"`
		}
		if json.Unmarshal(wsMsg.Data, &snap) != nil {
			continue
		}

		snapCount++
		fmt.Printf("--- Snapshot #%d (topic=%s) ---\n", snapCount, wsMsg.Topic)

		fmt.Printf("BIDS (%d levels):\n", len(snap.Bids))
		for i, b := range snap.Bids {
			tag := ""
			if i == 0 {
				tag = " ← [0] FIRST"
			}
			if i == len(snap.Bids)-1 {
				tag = " ← [last] LAST"
				if i == 0 {
					tag = " ← [0] FIRST & LAST"
				}
			}
			fmt.Printf("  [%d] price=%-10.4f size=%-12.2f%s\n", i, b[0], b[1], tag)
		}

		fmt.Printf("ASKS (%d levels):\n", len(snap.Asks))
		for i, a := range snap.Asks {
			tag := ""
			if i == 0 {
				tag = " ← [0] FIRST"
			}
			if i == len(snap.Asks)-1 {
				tag = " ← [last] LAST"
				if i == 0 {
					tag = " ← [0] FIRST & LAST"
				}
			}
			fmt.Printf("  [%d] price=%-10.4f size=%-12.2f%s\n", i, a[0], a[1], tag)
		}

		// Analysis
		if len(snap.Bids) >= 2 {
			if snap.Bids[0][0] > snap.Bids[len(snap.Bids)-1][0] {
				fmt.Printf("  → Bids DESCENDING → best bid = FIRST [0]\n")
			} else {
				fmt.Printf("  → Bids ASCENDING → best bid = LAST\n")
			}
		}
		if len(snap.Asks) >= 2 {
			if snap.Asks[0][0] < snap.Asks[len(snap.Asks)-1][0] {
				fmt.Printf("  → Asks ASCENDING → best ask = FIRST [0]\n")
			} else {
				fmt.Printf("  → Asks DESCENDING → best ask = LAST\n")
			}
		}

		if len(snap.Bids) > 0 && len(snap.Asks) > 0 {
			fmt.Printf("\n  Spread: Bids[0]=%.4f vs Asks[0]=%.4f → spread=%.4f %s\n",
				snap.Bids[0][0], snap.Asks[0][0], snap.Asks[0][0]-snap.Bids[0][0],
				iff(snap.Bids[0][0] < snap.Asks[0][0], "✅ NORMAL", "❌ CROSSED"))
		}
		fmt.Println()
	}
}

func iff(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
