package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"predict-market/internal/server"
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find executable: %v\n", err)
		os.Exit(1)
	}
	baseDir := filepath.Dir(filepath.Dir(exePath))
	if filepath.Base(filepath.Dir(exePath)) != "bin" {
		baseDir, _ = os.Getwd()
	}

	siteDir := filepath.Join(baseDir, "site")
	dataPath := filepath.Join(siteDir, "data", "markets_pairs.json")
	fetchBin := filepath.Join(filepath.Dir(exePath), "fetch_all_markets")

	port := envInt("PORT", 8050)
	refreshMs := envInt("REFRESH_INTERVAL_MS", 90000)
	ssePingMs := envInt("SSE_PING_MS", 25000)
	autoRefresh := envBool("AUTO_REFRESH", true)

	fetchArgs := []string{
		"--concurrency", envOr("FETCH_CONCURRENCY", "3"),
		"--category-concurrency", envOr("CATEGORY_CONCURRENCY", "2"),
	}
	if v := os.Getenv("MAX_MARKETS"); v != "" {
		fetchArgs = append(fetchArgs, "--max-markets", v)
	}
	if v := os.Getenv("FETCH_SLEEP"); v != "" {
		fetchArgs = append(fetchArgs, "--sleep", v)
	}
	if v := os.Getenv("COMMENTS_LIMIT"); v != "" {
		fetchArgs = append(fetchArgs, "--comments-limit", v)
	}
	if v := os.Getenv("HOLDERS_LIMIT"); v != "" {
		fetchArgs = append(fetchArgs, "--holders-limit", v)
	}
	if v := os.Getenv("REPLIES_LIMIT"); v != "" {
		fetchArgs = append(fetchArgs, "--replies-limit", v)
	}
	if os.Getenv("SKIP_ORDERBOOK") == "1" {
		fetchArgs = append(fetchArgs, "--skip-orderbook")
	}
	if os.Getenv("SKIP_HOLDERS") == "1" {
		fetchArgs = append(fetchArgs, "--skip-holders")
	}
	if os.Getenv("SKIP_COMMENTS") == "1" {
		fetchArgs = append(fetchArgs, "--skip-comments")
	}
	if os.Getenv("SKIP_TIMESERIES") == "1" {
		fetchArgs = append(fetchArgs, "--skip-timeseries")
	}

	cfg := server.ServerConfig{
		Port:              port,
		RefreshIntervalMs: refreshMs,
		SSEPingMs:         ssePingMs,
		AutoRefresh:       autoRefresh,
		SiteDir:           siteDir,
		DataPath:          dataPath,
		BaseDir:           baseDir,
		FetchBin:          fetchBin,
		FetchArgs:         fetchArgs,
		PolyBook: server.PolyBookConfig{
			ClobURL:     envOr("POLYMARKET_CLOB_URL", "https://clob.polymarket.com"),
			TTLMs:       envInt("POLYMARKET_ORDERBOOK_TTL_MS", 15000),
			Concurrency: envInt("POLYMARKET_ORDERBOOK_CONCURRENCY", 4),
			Levels:      envInt("POLYMARKET_ORDERBOOK_LEVELS", 8),
			MaxTokens:   envInt("POLYMARKET_ORDERBOOK_MAX_TOKENS", 6),
		},
	}

	srv := server.NewServer(cfg)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
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

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v != "0"
}
