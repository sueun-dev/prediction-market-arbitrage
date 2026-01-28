package config

import "strings"

// Config holds all CLI configuration options.
type Config struct {
	PageSize             int
	Sort                 string
	SleepSeconds         float64
	TimeoutSeconds       float64
	Retries              int
	BackoffSeconds       float64
	StatusFilter         string
	IncludeNonBettable   bool
	ViewOutPath          string
	FullOutPath          string
	RawOutPath           string
	IncludeOrderbook     bool
	IncludeHolders       bool
	IncludeComments      bool
	IncludeTimeseries    bool
	CommentsLimit        int
	HoldersLimit         int
	RepliesLimit         int
	FetchAllReplies      bool
	OrderbookTimeoutMs   int
	MaxMarkets           int
	Concurrency          int
	CategoryConcurrency  int
	TimeseriesIntervals  []string
	AuthToken            string
	Cookie               string
}

// Defaults returns a Config with default values.
func Defaults() Config {
	return Config{
		PageSize:            50,
		Sort:                "VOLUME_24H_DESC",
		SleepSeconds:        0,
		TimeoutSeconds:      20,
		Retries:             3,
		BackoffSeconds:      0.5,
		StatusFilter:        "",
		IncludeNonBettable:  false,
		ViewOutPath:         "site/data/markets_view.json",
		FullOutPath:         "site/data/markets_full.json",
		RawOutPath:          "",
		IncludeOrderbook:    true,
		IncludeHolders:      true,
		IncludeComments:     true,
		IncludeTimeseries:   true,
		CommentsLimit:       0,
		HoldersLimit:        0,
		RepliesLimit:        20,
		FetchAllReplies:     false,
		OrderbookTimeoutMs:  8000,
		MaxMarkets:          0,
		Concurrency:         20,
		CategoryConcurrency: 6,
		TimeseriesIntervals: []string{"_1D", "_7D", "_30D", "MAX"},
		AuthToken:           "",
		Cookie:              "",
	}
}

// ParseArgs parses CLI arguments into a Config.
func ParseArgs(argv []string) Config {
	cfg := Defaults()

	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if !strings.HasPrefix(arg, "--") {
			continue
		}

		key, inlineValue := splitArg(arg)
		value := inlineValue
		if value == "" && i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
			value = argv[i+1]
		}

		advance := inlineValue == "" && value != ""

		switch key {
		case "--page-size":
			cfg.PageSize = atoi(value, cfg.PageSize)
			if advance { i++ }
		case "--sort":
			if value != "" { cfg.Sort = value }
			if advance { i++ }
		case "--sleep":
			cfg.SleepSeconds = atof(value, cfg.SleepSeconds)
			if advance { i++ }
		case "--timeout":
			cfg.TimeoutSeconds = atof(value, cfg.TimeoutSeconds)
			if advance { i++ }
		case "--retries":
			cfg.Retries = atoi(value, cfg.Retries)
			if advance { i++ }
		case "--backoff":
			cfg.BackoffSeconds = atof(value, cfg.BackoffSeconds)
			if advance { i++ }
		case "--status":
			if value != "" { cfg.StatusFilter = value }
			if advance { i++ }
		case "--include-nonbettable":
			cfg.IncludeNonBettable = true
		case "--out":
			if value != "" { cfg.ViewOutPath = value }
			if advance { i++ }
		case "--full-out":
			if value != "" { cfg.FullOutPath = value }
			if advance { i++ }
		case "--raw-out":
			if value != "" { cfg.RawOutPath = value }
			if advance { i++ }
		case "--skip-orderbook":
			cfg.IncludeOrderbook = false
		case "--skip-holders":
			cfg.IncludeHolders = false
		case "--skip-comments":
			cfg.IncludeComments = false
		case "--skip-timeseries":
			cfg.IncludeTimeseries = false
		case "--comments-limit":
			cfg.CommentsLimit = atoi(value, cfg.CommentsLimit)
			if advance { i++ }
		case "--holders-limit":
			cfg.HoldersLimit = atoi(value, cfg.HoldersLimit)
			if advance { i++ }
		case "--replies-limit":
			cfg.RepliesLimit = atoi(value, cfg.RepliesLimit)
			if advance { i++ }
		case "--fetch-all-replies":
			cfg.FetchAllReplies = true
		case "--orderbook-timeout":
			cfg.OrderbookTimeoutMs = atoi(value, cfg.OrderbookTimeoutMs)
			if advance { i++ }
		case "--max-markets":
			cfg.MaxMarkets = atoi(value, cfg.MaxMarkets)
			if advance { i++ }
		case "--concurrency":
			cfg.Concurrency = atoi(value, cfg.Concurrency)
			if advance { i++ }
		case "--category-concurrency":
			cfg.CategoryConcurrency = atoi(value, cfg.CategoryConcurrency)
			if advance { i++ }
		case "--timeseries-intervals":
			if value != "" {
				parts := strings.Split(value, ",")
				var intervals []string
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						intervals = append(intervals, p)
					}
				}
				cfg.TimeseriesIntervals = intervals
			}
			if advance { i++ }
		case "--auth":
			cfg.AuthToken = value
			if advance { i++ }
		case "--cookie":
			cfg.Cookie = value
			if advance { i++ }
		}
	}

	return cfg
}

func splitArg(arg string) (string, string) {
	idx := strings.Index(arg, "=")
	if idx < 0 {
		return arg, ""
	}
	return arg[:idx], arg[idx+1:]
}

func atoi(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n := 0
	neg := false
	start := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		start = 1
	}
	for i := start; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		return -n
	}
	return n
}

func atof(s string, fallback float64) float64 {
	if s == "" {
		return fallback
	}
	// Simple float parser
	neg := false
	idx := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		idx = 1
	}
	intPart := 0.0
	for idx < len(s) && s[idx] >= '0' && s[idx] <= '9' {
		intPart = intPart*10 + float64(s[idx]-'0')
		idx++
	}
	fracPart := 0.0
	if idx < len(s) && s[idx] == '.' {
		idx++
		divisor := 10.0
		for idx < len(s) && s[idx] >= '0' && s[idx] <= '9' {
			fracPart += float64(s[idx]-'0') / divisor
			divisor *= 10
			idx++
		}
	}
	result := intPart + fracPart
	if neg {
		result = -result
	}
	return result
}
