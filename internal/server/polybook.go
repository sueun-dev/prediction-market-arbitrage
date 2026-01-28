package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"predict-market/internal/market"
	"predict-market/internal/polymarket"
	"predict-market/internal/worker"
)

// PolyBookConfig holds Polymarket orderbook proxy config.
type PolyBookConfig struct {
	ClobURL     string
	TTLMs       int
	Concurrency int
	Levels      int
	MaxTokens   int
}

type orderbookCacheEntry struct {
	data      market.OrderbookView
	expiresAt time.Time
}

// PolyBookProxy handles Polymarket orderbook proxy requests.
type PolyBookProxy struct {
	mu    sync.RWMutex
	cache map[string]orderbookCacheEntry
	cfg   PolyBookConfig
}

// NewPolyBookProxy creates a new proxy.
func NewPolyBookProxy(cfg PolyBookConfig) *PolyBookProxy {
	return &PolyBookProxy{
		cache: make(map[string]orderbookCacheEntry),
		cfg:   cfg,
	}
}

// OrderbookResult is the result for a single token.
type OrderbookResult struct {
	TokenID   string               `json:"tokenId"`
	OK        bool                 `json:"ok"`
	Orderbook *market.OrderbookView `json:"orderbook,omitempty"`
	Error     string               `json:"error,omitempty"`
}

// HandleOrderbook handles the /api/polymarket/orderbook endpoint.
func (p *PolyBookProxy) HandleOrderbook(w http.ResponseWriter, r *http.Request) {
	tokenParam := r.URL.Query().Get("token_ids")
	if tokenParam == "" {
		tokenParam = r.URL.Query().Get("token_id")
	}
	if tokenParam == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "token_ids is required"})
		return
	}

	parts := strings.Split(tokenParam, ",")
	var tokenIDs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tokenIDs = append(tokenIDs, p)
		}
	}
	if len(tokenIDs) > p.cfg.MaxTokens {
		tokenIDs = tokenIDs[:p.cfg.MaxTokens]
	}

	ctx := r.Context()
	results, _ := worker.Run(ctx, tokenIDs, p.cfg.Concurrency, func(ctx context.Context, tokenID string) (OrderbookResult, error) {
		book, err := p.fetchWithCache(ctx, tokenID)
		if err != nil {
			return OrderbookResult{TokenID: tokenID, OK: false, Error: err.Error()}, nil
		}
		return OrderbookResult{TokenID: tokenID, OK: true, Orderbook: &book}, nil
	})

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": results})
}

func (p *PolyBookProxy) fetchWithCache(ctx context.Context, tokenID string) (market.OrderbookView, error) {
	p.mu.RLock()
	entry, ok := p.cache[tokenID]
	p.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.data, nil
	}

	payload, err := polymarket.FetchOrderbook(ctx, p.cfg.ClobURL, tokenID)
	if err != nil {
		return market.OrderbookView{}, err
	}
	if payload.Error != nil {
		return market.OrderbookView{}, &orderbookError{msg: *payload.Error}
	}

	book := polymarket.NormalizeBook(payload, p.cfg.Levels)

	p.mu.Lock()
	p.cache[tokenID] = orderbookCacheEntry{
		data:      book,
		expiresAt: time.Now().Add(time.Duration(p.cfg.TTLMs) * time.Millisecond),
	}
	p.mu.Unlock()

	return book, nil
}

type orderbookError struct {
	msg string
}

func (e *orderbookError) Error() string {
	return e.msg
}
