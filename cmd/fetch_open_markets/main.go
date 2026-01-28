package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"predict-market/internal/config"
	"predict-market/internal/market"
	"predict-market/internal/predict"
	"predict-market/internal/worker"
)

func main() {
	cfg := config.ParseArgs(os.Args[1:])
	ctx := context.Background()

	// 1. Stream index → filter → limit → detail fetch: fully pipelined
	indexCh, indexErrCh := predict.StreamMarketIndex(ctx, cfg)

	// Channel for markets ready to be detail-fetched
	detailCh := make(chan predict.MarketIndex, cfg.Concurrency*2)
	var limited []predict.MarketIndex
	var limitedMu sync.Mutex
	var indexErr error

	// Filter and limit in a goroutine, feeding detailCh
	go func() {
		defer close(detailCh)
		count := 0
		for mi := range indexCh {
			if !cfg.IncludeNonBettable {
				if !mi.IsTradingEnabled {
					continue
				}
				if cfg.StatusFilter != "" && mi.Status != cfg.StatusFilter {
					continue
				}
			}
			limitedMu.Lock()
			limited = append(limited, mi)
			limitedMu.Unlock()

			detailCh <- mi
			count++
			if cfg.MaxMarkets > 0 && count >= cfg.MaxMarkets {
				break
			}
		}
		// Drain remaining
		go func() {
			for range indexCh {
			}
		}()
		indexErr = <-indexErrCh
	}()

	// 2. Concurrent detail fetches — start immediately as items arrive in detailCh
	type detailResult struct {
		idx    int
		market market.NormalizedMarket
	}

	var resultsMu sync.Mutex
	var results []detailResult
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Concurrency)
	var detailIdx int64

	for mi := range detailCh {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore
		idx := int(atomic.AddInt64(&detailIdx, 1) - 1)
		go func(mi predict.MarketIndex, idx int) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore

			detail, err := predict.FetchMarketDetail(ctx, cfg, mi.ID)
			if err != nil {
				return
			}

			var orderbook *market.OrderbookView
			if cfg.IncludeOrderbook {
				snapshot, _ := predict.FetchOrderbookSnapshot(ctx, mi.ID, cfg.OrderbookTimeoutMs)
				orderbook = predict.NormalizeOrderbook(snapshot)
			}

			var holders *market.MarketHolders
			if cfg.IncludeHolders {
				holders, _ = predict.FetchMarketHolders(ctx, cfg, detail)
			}

			m := predict.ToFullView(detail, orderbook, holders, nil, nil)
			resultsMu.Lock()
			results = append(results, detailResult{idx: idx, market: m})
			resultsMu.Unlock()
		}(mi, idx)
	}
	wg.Wait()

	if indexErr != nil {
		fmt.Fprintf(os.Stderr, "error fetching index: %v\n", indexErr)
		os.Exit(1)
	}

	// Sort results by original index to maintain order
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].idx < results[j-1].idx; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	sortedMarkets := make([]market.NormalizedMarket, len(results))
	for i := range results {
		sortedMarkets[i] = results[i].market
	}

	// 3. Category-level fetches in parallel
	limitedMu.Lock()
	limitedCopy := limited
	limitedMu.Unlock()

	categorySet := make(map[string]bool)
	for _, m := range limitedCopy {
		categorySet[m.Category.ID] = true
	}
	var categoryIDs []string
	for id := range categorySet {
		categoryIDs = append(categoryIDs, id)
	}

	commentsByCategory := &sync.Map{}
	timeseriesByCat := &sync.Map{}
	var catWg sync.WaitGroup

	if cfg.IncludeComments {
		catWg.Add(1)
		go func() {
			defer catWg.Done()
			worker.Run(ctx, categoryIDs, cfg.CategoryConcurrency, func(ctx context.Context, id string) (struct{}, error) {
				comments, err := predict.FetchComments(ctx, cfg, id)
				if err == nil {
					commentsByCategory.Store(id, comments)
				}
				return struct{}{}, err
			})
		}()
	}

	if cfg.IncludeTimeseries {
		catWg.Add(1)
		go func() {
			defer catWg.Done()
			worker.Run(ctx, categoryIDs, cfg.CategoryConcurrency, func(ctx context.Context, id string) (struct{}, error) {
				marketMap := make(map[string]map[string]*market.TimeseriesData)
				for _, interval := range cfg.TimeseriesIntervals {
					dataByMarket, err := predict.FetchCategoryTimeseries(ctx, cfg, id, interval)
					if err != nil {
						continue
					}
					for marketID, data := range dataByMarket {
						if marketMap[marketID] == nil {
							marketMap[marketID] = make(map[string]*market.TimeseriesData)
						}
						marketMap[marketID][interval] = data
					}
				}
				timeseriesByCat.Store(id, marketMap)
				return struct{}{}, nil
			})
		}()
	}

	catWg.Wait()

	// 4. Attach category data and filter
	var validMarkets []market.NormalizedMarket
	for _, m := range sortedMarkets {
		if m.ID == "" {
			continue
		}
		if val, ok := commentsByCategory.Load(m.Category.ID); ok {
			m.Comments = val.(*market.CategoryComments)
		}
		if val, ok := timeseriesByCat.Load(m.Category.ID); ok {
			catMap := val.(map[string]map[string]*market.TimeseriesData)
			if ts, ok := catMap[m.ID]; ok {
				m.Timeseries = ts
			}
		}
		validMarkets = append(validMarkets, m)
	}

	// 5. Write outputs
	now := formatISOTime(time.Now().UTC())

	fullPayload := market.FullPayload{
		GeneratedAt: now,
		Count:       len(validMarkets),
		Markets:     validMarkets,
	}
	if err := writeJSON(cfg.FullOutPath, fullPayload); err != nil {
		fmt.Fprintf(os.Stderr, "error writing full: %v\n", err)
		os.Exit(1)
	}

	viewMarkets := make([]market.ViewMarket, len(validMarkets))
	for i, m := range validMarkets {
		viewMarkets[i] = predict.ToView(m)
	}
	viewPayload := market.ViewPayload{
		GeneratedAt: now,
		Count:       len(validMarkets),
		Markets:     viewMarkets,
	}
	if err := writeJSON(cfg.ViewOutPath, viewPayload); err != nil {
		fmt.Fprintf(os.Stderr, "error writing view: %v\n", err)
		os.Exit(1)
	}

	if cfg.RawOutPath != "" {
		rawPayload := struct {
			GeneratedAt string                `json:"generatedAt"`
			Count       int                   `json:"count"`
			Markets     []predict.MarketIndex `json:"markets"`
		}{
			GeneratedAt: now,
			Count:       len(limitedCopy),
			Markets:     limitedCopy,
		}
		if err := writeJSON(cfg.RawOutPath, rawPayload); err != nil {
			fmt.Fprintf(os.Stderr, "error writing raw: %v\n", err)
			os.Exit(1)
		}
	}

	statusLabel := cfg.StatusFilter
	if statusLabel == "" {
		statusLabel = "ANY"
	}
	fmt.Fprintf(os.Stderr, "fetched=%d sort=%s status=%s fullOut=%s\n",
		len(validMarkets), cfg.Sort, statusLabel, cfg.FullOutPath)
}

func writeJSON(path string, payload interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func formatISOTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.000Z")
}
