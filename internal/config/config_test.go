package config

import "testing"

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.PageSize != 50 {
		t.Errorf("PageSize = %d, want 50", cfg.PageSize)
	}
	if cfg.Sort != "VOLUME_24H_DESC" {
		t.Errorf("Sort = %q, want VOLUME_24H_DESC", cfg.Sort)
	}
	if cfg.Concurrency != 20 {
		t.Errorf("Concurrency = %d, want 20", cfg.Concurrency)
	}
	if cfg.IncludeOrderbook != true {
		t.Error("IncludeOrderbook should default to true")
	}
	if len(cfg.TimeseriesIntervals) != 4 {
		t.Errorf("TimeseriesIntervals len = %d, want 4", len(cfg.TimeseriesIntervals))
	}
}

func TestParseArgs(t *testing.T) {
	args := []string{
		"--page-size", "100",
		"--sort", "NEWEST",
		"--sleep", "1.5",
		"--timeout", "30",
		"--retries", "5",
		"--backoff", "2.0",
		"--status", "OPEN",
		"--include-nonbettable",
		"--out", "/tmp/view.json",
		"--full-out", "/tmp/full.json",
		"--raw-out", "/tmp/raw.json",
		"--skip-orderbook",
		"--skip-holders",
		"--skip-comments",
		"--skip-timeseries",
		"--comments-limit", "10",
		"--holders-limit", "20",
		"--replies-limit", "5",
		"--fetch-all-replies",
		"--orderbook-timeout", "5000",
		"--max-markets", "50",
		"--concurrency", "8",
		"--category-concurrency", "5",
		"--timeseries-intervals", "_1D,_7D",
		"--auth", "mytoken",
		"--cookie", "mycookie",
	}

	cfg := ParseArgs(args)

	if cfg.PageSize != 100 {
		t.Errorf("PageSize = %d, want 100", cfg.PageSize)
	}
	if cfg.Sort != "NEWEST" {
		t.Errorf("Sort = %q, want NEWEST", cfg.Sort)
	}
	if cfg.SleepSeconds != 1.5 {
		t.Errorf("SleepSeconds = %f, want 1.5", cfg.SleepSeconds)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %f, want 30", cfg.TimeoutSeconds)
	}
	if cfg.Retries != 5 {
		t.Errorf("Retries = %d, want 5", cfg.Retries)
	}
	if cfg.BackoffSeconds != 2.0 {
		t.Errorf("BackoffSeconds = %f, want 2.0", cfg.BackoffSeconds)
	}
	if cfg.StatusFilter != "OPEN" {
		t.Errorf("StatusFilter = %q, want OPEN", cfg.StatusFilter)
	}
	if !cfg.IncludeNonBettable {
		t.Error("IncludeNonBettable should be true")
	}
	if cfg.ViewOutPath != "/tmp/view.json" {
		t.Errorf("ViewOutPath = %q, want /tmp/view.json", cfg.ViewOutPath)
	}
	if cfg.FullOutPath != "/tmp/full.json" {
		t.Errorf("FullOutPath = %q, want /tmp/full.json", cfg.FullOutPath)
	}
	if cfg.RawOutPath != "/tmp/raw.json" {
		t.Errorf("RawOutPath = %q, want /tmp/raw.json", cfg.RawOutPath)
	}
	if cfg.IncludeOrderbook {
		t.Error("IncludeOrderbook should be false")
	}
	if cfg.IncludeHolders {
		t.Error("IncludeHolders should be false")
	}
	if cfg.IncludeComments {
		t.Error("IncludeComments should be false")
	}
	if cfg.IncludeTimeseries {
		t.Error("IncludeTimeseries should be false")
	}
	if cfg.CommentsLimit != 10 {
		t.Errorf("CommentsLimit = %d, want 10", cfg.CommentsLimit)
	}
	if cfg.HoldersLimit != 20 {
		t.Errorf("HoldersLimit = %d, want 20", cfg.HoldersLimit)
	}
	if cfg.RepliesLimit != 5 {
		t.Errorf("RepliesLimit = %d, want 5", cfg.RepliesLimit)
	}
	if !cfg.FetchAllReplies {
		t.Error("FetchAllReplies should be true")
	}
	if cfg.OrderbookTimeoutMs != 5000 {
		t.Errorf("OrderbookTimeoutMs = %d, want 5000", cfg.OrderbookTimeoutMs)
	}
	if cfg.MaxMarkets != 50 {
		t.Errorf("MaxMarkets = %d, want 50", cfg.MaxMarkets)
	}
	if cfg.Concurrency != 8 {
		t.Errorf("Concurrency = %d, want 8", cfg.Concurrency)
	}
	if cfg.CategoryConcurrency != 5 {
		t.Errorf("CategoryConcurrency = %d, want 5", cfg.CategoryConcurrency)
	}
	if len(cfg.TimeseriesIntervals) != 2 || cfg.TimeseriesIntervals[0] != "_1D" || cfg.TimeseriesIntervals[1] != "_7D" {
		t.Errorf("TimeseriesIntervals = %v, want [_1D, _7D]", cfg.TimeseriesIntervals)
	}
	if cfg.AuthToken != "mytoken" {
		t.Errorf("AuthToken = %q, want mytoken", cfg.AuthToken)
	}
	if cfg.Cookie != "mycookie" {
		t.Errorf("Cookie = %q, want mycookie", cfg.Cookie)
	}
}

func TestParseArgsInlineEquals(t *testing.T) {
	args := []string{
		"--page-size=200",
		"--sort=NEWEST",
		"--max-markets=10",
	}

	cfg := ParseArgs(args)

	if cfg.PageSize != 200 {
		t.Errorf("PageSize = %d, want 200", cfg.PageSize)
	}
	if cfg.Sort != "NEWEST" {
		t.Errorf("Sort = %q, want NEWEST", cfg.Sort)
	}
	if cfg.MaxMarkets != 10 {
		t.Errorf("MaxMarkets = %d, want 10", cfg.MaxMarkets)
	}
}

func TestParseArgsEmpty(t *testing.T) {
	cfg := ParseArgs(nil)
	defaults := Defaults()

	if cfg.PageSize != defaults.PageSize {
		t.Errorf("PageSize = %d, want %d", cfg.PageSize, defaults.PageSize)
	}
	if cfg.Concurrency != defaults.Concurrency {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, defaults.Concurrency)
	}
}
