package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"predict-market/internal/config"
	"predict-market/internal/market"
	"predict-market/internal/matcher"
	"predict-market/internal/polymarket"
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find executable: %v\n", err)
		os.Exit(1)
	}
	baseDir := filepath.Dir(filepath.Dir(exePath))
	// If running from bin/, go up one level; if running directly, use cwd
	if filepath.Base(filepath.Dir(exePath)) == "bin" {
		baseDir = filepath.Dir(filepath.Dir(exePath))
	} else {
		baseDir, _ = os.Getwd()
	}

	dataDir := filepath.Join(baseDir, "site", "data")
	predictFull := filepath.Join(dataDir, "predict_markets_full.json")
	predictView := filepath.Join(dataDir, "predict_markets_view.json")
	outFull := filepath.Join(dataDir, "markets_full.json")
	outView := filepath.Join(dataDir, "markets_view.json")
	pairsOut := filepath.Join(dataDir, "markets_pairs.json")

	// Env config
	polyAPI := envOr("POLYMARKET_API", "https://gamma-api.polymarket.com")
	polyPageSize := envInt("POLYMARKET_PAGE_SIZE", 100)
	polyMaxMarkets := resolvePolymarketMaxMarkets(os.Args[1:])
	polyActiveOnly := envBool("POLYMARKET_ACTIVE_ONLY", true)
	polyAcceptingOnly := envBool("POLYMARKET_ACCEPTING_ONLY", true)
	pairMinSimilarity := envFloat("PAIR_MIN_SIMILARITY", 0.8)
	pairMinCharSimilarity := envFloat("PAIR_MIN_CHAR_SIMILARITY", 0.78)
	pairMinMargin := envFloat("PAIR_MIN_MARGIN", 0.08)
	pairMinTokens := envInt("PAIR_MIN_TOKENS", 4)
	pairRequireNumberMatch := envBool("PAIR_REQUIRE_NUMBER_MATCH", true)
	pairRequireYearMatch := envBool("PAIR_REQUIRE_YEAR_MATCH", true)
	pairRequireMonthMatch := envBool("PAIR_REQUIRE_MONTH_MATCH", true)
	pairRequireSubjectMatch := envBool("PAIR_REQUIRE_SUBJECT_MATCH", true)
	pairRequireDescDateMatch := envBool("PAIR_REQUIRE_DESC_DATE_MATCH", true)

	// Filter CLI args: remove --full-out and --out since we set them
	argv := filterPredictArgs(os.Args[1:])

	// 1. Run fetch_open_markets
	fetchBin := filepath.Join(filepath.Dir(exePath), "fetch_open_markets")
	fetchArgs := append([]string{
		"--full-out", predictFull,
		"--out", predictView,
	}, argv...)

	cmd := exec.Command(fetchBin, fetchArgs...)
	cmd.Dir = baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fetch_open_markets failed: %v\n", err)
		os.Exit(1)
	}

	// 2. Read predict data
	predictData, err := os.ReadFile(predictFull)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read predict full: %v\n", err)
		os.Exit(1)
	}
	var predictPayload market.FullPayload
	if err := json.Unmarshal(predictData, &predictPayload); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal predict: %v\n", err)
		os.Exit(1)
	}

	predictMarkets := predictPayload.Markets
	for i := range predictMarkets {
		predictMarkets[i].Source = "Predict.Fun"
		predictMarkets[i].SourceUrl = "https://predict.fun/"
	}

	// 3. Fetch Polymarket
	ctx := context.Background()
	var polymarketMarkets []market.NormalizedMarket
	polyCfg := polymarket.ClientConfig{
		APIBase:       polyAPI,
		PageSize:      polyPageSize,
		MaxMarkets:    polyMaxMarkets,
		ActiveOnly:    polyActiveOnly,
		AcceptingOnly: polyAcceptingOnly,
	}
	polymarketMarkets, err = polymarket.FetchMarkets(ctx, polyCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "polymarket_error=%v\n", err)
		polymarketMarkets = nil
	}

	// 4. Combine
	combined := make([]market.NormalizedMarket, 0, len(predictMarkets)+len(polymarketMarkets))
	combined = append(combined, predictMarkets...)
	combined = append(combined, polymarketMarkets...)
	generatedAt := formatISOTime(time.Now().UTC())

	// 5. Write full
	if err := writeJSON(outFull, market.CombinedPayload{
		GeneratedAt: generatedAt,
		Count:       len(combined),
		Markets:     combined,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write full: %v\n", err)
		os.Exit(1)
	}

	// 6. Write view
	viewMarkets := make([]market.ViewMarket, len(combined))
	for i, m := range combined {
		viewOutcomes := make([]market.ViewOutcome, len(m.Outcomes))
		for j, o := range m.Outcomes {
			viewOutcomes[j] = market.ViewOutcome{
				ID:             o.ID,
				Name:           o.Name,
				Index:          o.Index,
				PositionsCount: o.PositionsCount,
			}
		}
		viewMarkets[i] = market.ViewMarket{
			ID:               m.ID,
			Title:            m.Title,
			Question:         m.Question,
			ImageUrl:         m.ImageUrl,
			Category:         m.Category,
			ChancePercentage: m.ChancePercentage,
			SpreadThreshold:  m.SpreadThreshold,
			SpreadDecimal:    strPtr(m.SpreadThresholdDecimal),
			SpreadPercent:    strPtr(m.SpreadThresholdPercent),
			MakerFeeBps:      &m.MakerFeeBps,
			TakerFeeBps:      &m.TakerFeeBps,
			IsTradingEnabled: &m.IsTradingEnabled,
			Status:           &m.Status,
			ShareThreshold:   &m.ShareThreshold,
			Statistics:       &m.Statistics,
			Outcomes:         viewOutcomes,
			TotalPositions:   m.TotalPositions,
			Source:           &m.Source,
		}
	}
	if err := writeJSON(outView, market.ViewPayload{
		GeneratedAt: generatedAt,
		Count:       len(combined),
		Markets:     viewMarkets,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write view: %v\n", err)
		os.Exit(1)
	}

	// 7. Build pairs
	matchCfg := matcher.MatchConfig{
		MinSimilarity:               pairMinSimilarity,
		MinCharSimilarity:           pairMinCharSimilarity,
		MinMargin:                   pairMinMargin,
		MinTokens:                   pairMinTokens,
		RequireNumberMatch:          pairRequireNumberMatch,
		RequireYearMatch:            pairRequireYearMatch,
		RequireMonthMatch:           pairRequireMonthMatch,
		RequireSubjectMatch:         pairRequireSubjectMatch,
		RequireDescriptionDateMatch: pairRequireDescDateMatch,
	}
	pairs := matcher.BuildPairs(predictMarkets, polymarketMarkets, matchCfg)
	if err := writeJSON(pairsOut, market.PairsPayload{
		GeneratedAt: generatedAt,
		Count:       len(pairs),
		Pairs:       pairs,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write pairs: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "combined=%d predict=%d polymarket=%d pairs=%d\n",
		len(combined), len(predictMarkets), len(polymarketMarkets), len(pairs))
}

func filterPredictArgs(argv []string) []string {
	var filtered []string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if strings.HasPrefix(arg, "--full-out") {
			if !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--out") {
			if !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func resolvePolymarketMaxMarkets(argv []string) int {
	if v := os.Getenv("POLYMARKET_MAX_MARKETS"); v != "" {
		return envInt("POLYMARKET_MAX_MARKETS", 0)
	}
	return config.ParseArgs(argv).MaxMarkets
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

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v != "0"
}
