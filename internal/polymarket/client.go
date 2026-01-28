package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"predict-market/internal/market"
)

// ClientConfig holds Polymarket API configuration.
type ClientConfig struct {
	APIBase        string
	PageSize       int
	MaxMarkets     int
	ActiveOnly     bool
	AcceptingOnly  bool
}

// FetchMarkets fetches all markets from the Polymarket gamma API.
func FetchMarkets(ctx context.Context, cfg ClientConfig) ([]market.NormalizedMarket, error) {
	var markets []market.NormalizedMarket
	offset := 0

	for {
		url := fmt.Sprintf("%s/markets?limit=%d&offset=%d",
			cfg.APIBase, cfg.PageSize, offset)
		if cfg.ActiveOnly {
			url += "&active=true&closed=false"
		}

		batch, err := fetchJSON[[]RawMarket](ctx, url)
		if err != nil {
			return nil, fmt.Errorf("polymarket fetch offset=%d: %w", offset, err)
		}
		if len(batch) == 0 {
			break
		}

		for _, raw := range batch {
			if cfg.AcceptingOnly && (raw.AcceptingOrders == nil || !*raw.AcceptingOrders) {
				continue
			}
			markets = append(markets, NormalizeMarket(raw))
			if cfg.MaxMarkets > 0 && len(markets) >= cfg.MaxMarkets {
				return markets, nil
			}
		}

		offset += len(batch)
		if len(batch) < cfg.PageSize {
			break
		}
	}

	return markets, nil
}

func fetchJSON[T any](ctx context.Context, url string) (T, error) {
	var zero T

	var lastErr error
	for attempt := 0; attempt <= 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(time.Duration(400*attempt) * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return zero, err
		}
		req.Header.Set("User-Agent", "predict-market")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			continue
		}

		var result T
		if err := json.Unmarshal(body, &result); err != nil {
			return zero, fmt.Errorf("unmarshal: %w", err)
		}
		return result, nil
	}

	return zero, fmt.Errorf("all retries exhausted: %w", lastErr)
}

// FetchOrderbook fetches an orderbook from the Polymarket CLOB API.
func FetchOrderbook(ctx context.Context, clobURL string, tokenID string) (*PolyBookPayload, error) {
	url := fmt.Sprintf("%s/book?token_id=%s", clobURL, tokenID)
	return fetchJSON[*PolyBookPayload](ctx, url)
}

// PolyBookPayload is the raw orderbook response from CLOB.
type PolyBookPayload struct {
	Timestamp *int64           `json:"timestamp"`
	Error     *string          `json:"error"`
	Asks      []PolyBookRow    `json:"asks"`
	Bids      []PolyBookRow    `json:"bids"`
}

// PolyBookRow is a price/size level from CLOB.
type PolyBookRow struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

// NormalizeBook converts a CLOB orderbook to a normalized view.
func NormalizeBook(payload *PolyBookPayload, levels int) market.OrderbookView {
	mapRow := func(r PolyBookRow) market.OrderbookRow {
		p, _ := strconv.ParseFloat(r.Price, 64)
		s, _ := strconv.ParseFloat(r.Size, 64)
		return market.OrderbookRow{Price: p, Size: s}
	}

	filterValid := func(rows []market.OrderbookRow) []market.OrderbookRow {
		var valid []market.OrderbookRow
		for _, r := range rows {
			if r.Price == r.Price && r.Size == r.Size { // NaN check
				valid = append(valid, r)
			}
		}
		return valid
	}

	asks := make([]market.OrderbookRow, len(payload.Asks))
	for i, a := range payload.Asks {
		asks[i] = mapRow(a)
	}
	asks = filterValid(asks)
	if len(asks) > levels {
		asks = asks[:levels]
	}

	bids := make([]market.OrderbookRow, len(payload.Bids))
	for i, b := range payload.Bids {
		bids[i] = mapRow(b)
	}
	bids = filterValid(bids)
	if len(bids) > levels {
		bids = bids[:levels]
	}

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

	return market.OrderbookView{
		UpdateTimestampMs: payload.Timestamp,
		BestAsk:           bestAsk,
		BestBid:           bestBid,
		Spread:            spread,
		SpreadCents:       spreadCents,
		Asks:              asks,
		Bids:              bids,
	}
}
