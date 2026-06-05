package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"predict-market/internal/market"
)

const (
	predictTakerFeeBps    int64 = 200
	polymarketTakerFeeBps int64 = 100
	priceScale            int64 = 1_000_000
	bpsScale              int64 = 10_000
)

type ScanConfig struct {
	MinNetBps    float64
	MinFillRatio float64

	// Feasibility / quality gates. An opportunity must clear ALL of these
	// (in addition to MinNetBps) before it is labeled "PROFITABLE" and ranked.
	MinAbsPrice    float64 // minimum buy price (reject sub-cent longshots whose bps are noise)
	MinNetPerShare float64 // minimum net $/share edge (reject tiny absolute edges)
	MaxQuoteSkewMs int64   // max allowed difference between the two legs' quote timestamps
}

// FillResult represents what happens when you try to fill a given USD amount
// against an orderbook side.
type FillResult struct {
	FilledShares float64
	FilledUSD    float64
	VWAP         float64
	MaxFillUSD   float64
	Levels       int
	Slippage     float64
}

// ArbOpportunity with depth-aware fields.
type ArbOpportunity struct {
	PairID       string
	Question     string
	Type         string
	BuyPlatform  string
	BuyPrice     float64
	SellPlatform string
	SellPrice    float64
	GrossProfit  float64
	NetProfit    float64
	NetBps       float64
	PredictLiq   float64
	PolyLiq      float64

	// Depth analysis
	BuyDepthUSD  float64
	SellDepthUSD float64
	MaxTradeUSD  float64
	BuyLevels    int
	SellLevels   int

	// Source data quality
	BuyPriceSrc  string // "ob" = orderbook bid/ask, "pricing" = pricing field
	SellPriceSrc string

	// Quote freshness: source timestamps for each leg and their absolute skew.
	BuyQuoteMs  int64
	SellQuoteMs int64
	QuoteSkewMs int64 // abs(BuyQuoteMs - SellQuoteMs); -1 if either timestamp is missing

	// Executable is true only when the opportunity clears every feasibility gate
	// (real fillable depth on BOTH legs, fresh quotes, and price/edge floors).
	// Only executable opportunities are labeled "PROFITABLE" and ranked.
	Executable    bool
	NotExecReason string

	// Simulated fill at various sizes
	Fills []SimFill
}

// SimFill simulates a round-trip trade at a given notional size.
type SimFill struct {
	SizeUSD     float64
	BuyVWAP     float64
	SellVWAP    float64
	GrossProfit float64
	NetProfit   float64
	NetBps      float64
	Feasible    bool
}

// resolvedPrice holds a price and where it came from.
type resolvedPrice struct {
	value  float64
	source string // "pricing", "ob", "mid"
}

func main() {
	path := "site/data/markets_pairs.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	var payload market.PairsPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d pairs (generated %s)\n\n", payload.Count, payload.GeneratedAt)

	scanCfg := loadScanConfig()

	simSizes := []float64{100, 500, 1000, 5000}

	var opps []ArbOpportunity
	totalChecks := 0

	for _, pair := range payload.Pairs {
		pp := pair.Pricing.Predict
		pm := pair.Pricing.Polymarket

		pLiq := pair.Predict.Statistics.TotalLiquidityUsd
		mLiq := pair.Polymarket.Statistics.TotalLiquidityUsd

		// Extract orderbook depth arrays (Predict has full depth, Poly usually empty)
		var pAsks, pBids []market.OrderbookRow
		if pair.Predict.Orderbook != nil {
			pAsks = pair.Predict.Orderbook.Asks
			pBids = pair.Predict.Orderbook.Bids
		}
		var mAsks, mBids []market.OrderbookRow
		if pair.Polymarket.Orderbook != nil {
			mAsks = pair.Polymarket.Orderbook.Asks
			mBids = pair.Polymarket.Orderbook.Bids
		}

		// Source timestamps for each platform's quotes (used for the freshness gate).
		pQuoteMs := orderbookTimestampMs(pair.Predict.Orderbook)
		mQuoteMs := orderbookTimestampMs(pair.Polymarket.Orderbook)

		// =====================================================================
		// PRICE RESOLUTION RULES (strictest possible)
		//
		// For an arb to be actionable:
		//   - BUY side needs an ASK price (the price someone is offering to sell)
		//   - SELL side needs a BID price (the price someone is willing to buy)
		//
		// Data hierarchy:
		//   1. pricing.yes.bid / pricing.yes.ask  (from API per-outcome)
		//   2. orderbook.bestBid / orderbook.bestAsk  (from orderbook snapshot)
		//   3. pricing.yes.price  (mid/last price — NOT executable, skip for arb)
		//
		// The NO-side pricing is synthetic (1 - YES). We do NOT use it for
		// cross arb. For complement arb, we derive the cost correctly.
		//
		// COMPLEMENT logic:
		//   Buy YES on platform A at ask_A + Buy NO on platform B.
		//   "Buy NO on B" = effectively pay (1 - yesBid_B) per share.
		//   This is because on a binary market: NO_ask = 1 - YES_bid.
		//   Total cost = ask_A + (1 - bid_B).
		//   Profit = 1.0 - total_cost_with_fees.
		//   This is equivalent to: bid_B - ask_A = YES cross gross.
		//   So COMPLEMENT and YES_CROSS are measuring the same spread
		//   just structured differently. We only report YES_CROSS to avoid
		//   double-counting.
		// =====================================================================

		// --- Predict YES prices (well-defined: API returns per-outcome bid/ask) ---
		pBid := resolvePrice(deref(pp.Yes.Bid), "pricing",
			derefOB(pair.Predict.Orderbook, true), "ob")
		pAsk := resolvePrice(deref(pp.Yes.Ask), "pricing",
			derefOB(pair.Predict.Orderbook, false), "ob")

		// --- Polymarket YES prices ---
		// Poly pricing.yes.bid/ask are usually null.
		// Orderbook bestBid/bestAsk are the real executable prices.
		// pricing.yes.price is a mid/last price — NOT executable.
		mBid := resolvePrice(deref(pm.Yes.Bid), "pricing",
			derefOB(pair.Polymarket.Orderbook, true), "ob")
		mAsk := resolvePrice(deref(pm.Yes.Ask), "pricing",
			derefOB(pair.Polymarket.Orderbook, false), "ob")

		// Sanity checks
		if pBid.value > 0 && pAsk.value > 0 && pBid.value >= pAsk.value {
			// bid >= ask on same platform = locked/crossed, skip
			continue
		}
		if mBid.value > 0 && mAsk.value > 0 && mBid.value >= mAsk.value {
			continue
		}

		// === YES Cross Direction A: Buy on Predict (at ask), Sell on Poly (at bid) ===
		// resolvePrice only ever yields "pricing" or "ob" sources (never "mid"),
		// so both legs here are executable price levels by construction.
		if pAsk.value > 0 && mBid.value > 0 {
			totalChecks++
			gross, net, netBps, ok := calcPerShareEdge(
				pAsk.value, mBid.value, predictTakerFeeBps, polymarketTakerFeeBps,
			)
			if ok {
				opp := ArbOpportunity{
					PairID: pair.ID, Question: pair.Question,
					Type: "YES_CROSS", BuyPlatform: "Predict", BuyPrice: pAsk.value,
					SellPlatform: "Polymarket", SellPrice: mBid.value,
					GrossProfit: gross, NetProfit: net, NetBps: netBps,
					PredictLiq: pLiq, PolyLiq: mLiq,
					BuyPriceSrc: pAsk.source, SellPriceSrc: mBid.source,
					BuyQuoteMs: pQuoteMs, SellQuoteMs: mQuoteMs,
				}
				fillDepth(&opp, pAsks, mBids, predictTakerFeeBps, polymarketTakerFeeBps, simSizes, scanCfg.MinFillRatio)
				applyFeasibilityGate(&opp, scanCfg)
				opps = append(opps, opp)
			}
		}

		// === YES Cross Direction B: Buy on Poly (at ask), Sell on Predict (at bid) ===
		if mAsk.value > 0 && pBid.value > 0 {
			totalChecks++
			gross, net, netBps, ok := calcPerShareEdge(
				mAsk.value, pBid.value, polymarketTakerFeeBps, predictTakerFeeBps,
			)
			if ok {
				opp := ArbOpportunity{
					PairID: pair.ID, Question: pair.Question,
					Type: "YES_CROSS", BuyPlatform: "Polymarket", BuyPrice: mAsk.value,
					SellPlatform: "Predict", SellPrice: pBid.value,
					GrossProfit: gross, NetProfit: net, NetBps: netBps,
					PredictLiq: pLiq, PolyLiq: mLiq,
					BuyPriceSrc: mAsk.source, SellPriceSrc: pBid.source,
					BuyQuoteMs: mQuoteMs, SellQuoteMs: pQuoteMs,
				}
				fillDepth(&opp, mAsks, pBids, polymarketTakerFeeBps, predictTakerFeeBps, simSizes, scanCfg.MinFillRatio)
				applyFeasibilityGate(&opp, scanCfg)
				opps = append(opps, opp)
			}
		}

		// NOTE: NO_CROSS and COMPLEMENT are skipped.
		//
		// NO_CROSS: NO pricing is synthetic (1 - YES), bid > ask = unusable.
		//
		// COMPLEMENT: Buy YES_A + Buy NO_B → $1.
		//   cost = ask_A + (1 - bid_B) = ask_A + 1 - bid_B
		//   profit = 1 - cost = bid_B - ask_A
		//   This is exactly the same as YES_CROSS gross profit.
		//   After fees the structure differs slightly (fees on 2 buys vs 1 buy + 1 sell),
		//   but since we don't have real NO orderbook depth, the execution is the same:
		//   you'd actually execute it as a YES cross anyway.
		//   So we avoid double-counting by only showing YES_CROSS.
	}

	// Sort executable opportunities ahead of non-executable ones, then by ROI
	// (net bps) descending. This keeps phantom/unfillable signals out of the
	// headline ranking even when their (untradeable) bps look enormous.
	sort.Slice(opps, func(i, j int) bool {
		if opps[i].Executable != opps[j].Executable {
			return opps[i].Executable
		}
		return opps[i].NetBps > opps[j].NetBps
	})

	green := "\033[32m"
	yellow := "\033[33m"
	red := "\033[31m"
	cyan := "\033[36m"
	reset := "\033[0m"
	bold := "\033[1m"
	dim := "\033[2m"

	profitable := 0
	for _, o := range opps {
		if isProfitable(o, scanCfg.MinNetBps) {
			profitable++
		}
	}

	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n", bold, reset)
	fmt.Printf("%s  ARB SCAN — YES CROSS ONLY (strict pricing)%s\n", bold, reset)
	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n\n", bold, reset)
	unexecutable := len(opps) - executableCount(opps)
	fmt.Printf("Pairs scanned:            %d\n", payload.Count)
	fmt.Printf("Actionable checks:        %d\n", totalChecks)
	fmt.Printf("Candidate opportunities:  %d\n", len(opps))
	fmt.Printf("Unexecutable (excluded):  %d\n", unexecutable)
	fmt.Printf("Profitable (>= %.1f bps):  %s%d%s\n", scanCfg.MinNetBps, green, profitable, reset)
	fmt.Printf("Fee model:                Predict %d bps | Polymarket %d bps\n", predictTakerFeeBps, polymarketTakerFeeBps)
	fmt.Printf("Fill ratio threshold:     %.1f%%\n", scanCfg.MinFillRatio*100)
	fmt.Printf("Feasibility gates:        min price $%.3f | min net $%.4f/sh | max quote skew %ds | depth on BOTH legs\n",
		scanCfg.MinAbsPrice, scanCfg.MinNetPerShare, scanCfg.MaxQuoteSkewMs/1000)
	fmt.Printf("Logic:                    Buy YES @ ask on A, Sell YES @ bid on B\n\n")

	if profitable == 0 {
		fmt.Printf("%sNo profitable opportunities found.%s\n\n", yellow, reset)
		// Show top near-miss
		fmt.Printf("Top 15 near-miss:\n\n")
		shown := 0
		for _, o := range opps {
			if shown >= 15 {
				break
			}
			printOpp(o, shown+1, green, yellow, red, cyan, reset, bold, dim, scanCfg.MinNetBps)
			shown++
		}
	} else {
		fmt.Printf("%s── PROFITABLE (%d) ──%s\n\n", green, profitable, reset)
		rank := 0
		for _, o := range opps {
			if !isProfitable(o, scanCfg.MinNetBps) {
				continue
			}
			rank++
			printOpp(o, rank, green, yellow, red, cyan, reset, bold, dim, scanCfg.MinNetBps)
		}
	}

	// Summary
	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n", bold, reset)
	fmt.Printf("%s  SUMMARY%s\n", bold, reset)
	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n\n", bold, reset)

	if len(opps) > 0 && isProfitable(opps[0], scanCfg.MinNetBps) {
		fmt.Printf("Best net/share:   %s$%+.4f (%.0f bps)%s\n", green, opps[0].NetProfit, opps[0].NetBps, reset)
	} else if len(opps) > 0 {
		note := "(below threshold)"
		if !opps[0].Executable && opps[0].NotExecReason != "" {
			note = "(not executable: " + opps[0].NotExecReason + ")"
		}
		fmt.Printf("Best net/share:   %s$%+.4f (%.0f bps)%s  %s\n", red, opps[0].NetProfit, opps[0].NetBps, reset, note)
	}
	fmt.Printf("Profitable:       %d / %d\n", profitable, len(opps))

	// Data quality note
	fmt.Printf("\n%sData quality notes:%s\n", bold, reset)
	obCount := 0
	pricingCount := 0
	for _, o := range opps {
		if !isProfitable(o, scanCfg.MinNetBps) {
			continue
		}
		if o.BuyPriceSrc == "ob" || o.SellPriceSrc == "ob" {
			obCount++
		}
		if o.BuyPriceSrc == "pricing" || o.SellPriceSrc == "pricing" {
			pricingCount++
		}
	}
	fmt.Printf("  Profitable using orderbook bid/ask: %d\n", obCount)
	fmt.Printf("  Profitable using pricing field:     %d\n", pricingCount)
	fmt.Printf("  Polymarket depth (asks/bids):       empty (bestBid/bestAsk only)\n")
	fmt.Printf("  Predict depth:                      full orderbook available\n")
}

func loadScanConfig() ScanConfig {
	cfg := ScanConfig{
		MinNetBps:    envFloat("ARB_MIN_NET_BPS", 15),
		MinFillRatio: envFloat("ARB_MIN_FILL_RATIO", 0.99),
		// Defaults chosen to suppress the known false-signal classes:
		//   - sub-cent longshots whose bps explode (e.g. 0.001 -> 0.003 = +19,109 bps)
		//   - vanishingly small absolute edges
		//   - legs quoted far apart in time (the two snapshots can be hours apart)
		MinAbsPrice:    envFloat("ARB_MIN_ABS_PRICE", 0.02),
		MinNetPerShare: envFloat("ARB_MIN_NET_PER_SHARE", 0.005),
		MaxQuoteSkewMs: int64(envFloat("ARB_MAX_QUOTE_SKEW_MS", 60_000)),
	}
	if cfg.MinFillRatio <= 0 || cfg.MinFillRatio > 1 {
		cfg.MinFillRatio = 0.99
	}
	if cfg.MinAbsPrice < 0 {
		cfg.MinAbsPrice = 0
	}
	if cfg.MinNetPerShare < 0 {
		cfg.MinNetPerShare = 0
	}
	if cfg.MaxQuoteSkewMs < 0 {
		cfg.MaxQuoteSkewMs = 0
	}
	return cfg
}

// orderbookTimestampMs returns the snapshot timestamp for a leg, or 0 if absent.
func orderbookTimestampMs(ob *market.OrderbookView) int64 {
	if ob == nil || ob.UpdateTimestampMs == nil {
		return 0
	}
	return *ob.UpdateTimestampMs
}

// hasFeasibleFill reports whether at least one simulated fill actually filled
// against real depth on BOTH legs.
func hasFeasibleFill(o *ArbOpportunity) bool {
	for _, f := range o.Fills {
		if f.Feasible {
			return true
		}
	}
	return false
}

// applyFeasibilityGate decides whether an opportunity is actually executable.
//
// This is the core fix for the "phantom"/"stale" false signals: a positive
// net-bps number is NOT sufficient to call an opportunity profitable. We
// additionally require, in order of cheap-to-expensive:
//
//	#3  absolute-price floor      — sub-cent longshots (e.g. 0.001 vs 0.003)
//	    + min net-$/share floor     produce enormous bps that are pure noise.
//	#2  quote-freshness            — both legs must be quoted within MaxQuoteSkewMs
//	    of each other (the two snapshots can otherwise be hours apart).
//	#1  real fillable depth        — BOTH legs need non-zero depth that actually
//	    on both legs                fills (Polymarket books arrive depth-less, so
//	                                a leg priced only off bestBid/bestAsk is not
//	                                tradeable at any size).
//
// The first failing check wins so the surfaced reason is the most fundamental.
func applyFeasibilityGate(o *ArbOpportunity, cfg ScanConfig) {
	// Compute quote skew (used both for the gate and for display).
	if o.BuyQuoteMs > 0 && o.SellQuoteMs > 0 {
		skew := o.BuyQuoteMs - o.SellQuoteMs
		if skew < 0 {
			skew = -skew
		}
		o.QuoteSkewMs = skew
	} else {
		o.QuoteSkewMs = -1
	}

	o.Executable = false

	// #3: absolute price floor.
	if o.BuyPrice < cfg.MinAbsPrice {
		o.NotExecReason = fmt.Sprintf("buy price %.4f < min %.3f", o.BuyPrice, cfg.MinAbsPrice)
		return
	}
	// #3: minimum net-per-share floor.
	if o.NetProfit < cfg.MinNetPerShare {
		o.NotExecReason = fmt.Sprintf("net $%.4f/sh < min $%.4f", o.NetProfit, cfg.MinNetPerShare)
		return
	}
	// #2: quote freshness.
	if o.QuoteSkewMs < 0 {
		o.NotExecReason = "missing quote timestamp on a leg"
		return
	}
	if o.QuoteSkewMs > cfg.MaxQuoteSkewMs {
		o.NotExecReason = fmt.Sprintf("stale: legs %ds apart (max %ds)", o.QuoteSkewMs/1000, cfg.MaxQuoteSkewMs/1000)
		return
	}
	// #1: real fillable depth on BOTH legs.
	if o.MaxTradeUSD <= 0 || !hasFeasibleFill(o) {
		o.NotExecReason = "no fillable depth on both legs"
		return
	}

	o.Executable = true
	o.NotExecReason = ""
}

// isProfitable reports whether an opportunity should appear in the headline
// "PROFITABLE" list: it must be executable AND clear the net-bps threshold.
func isProfitable(o ArbOpportunity, minNetBps float64) bool {
	return o.Executable && o.NetBps >= minNetBps
}

// executableCount returns how many opportunities passed every feasibility gate.
func executableCount(opps []ArbOpportunity) int {
	n := 0
	for _, o := range opps {
		if o.Executable {
			n++
		}
	}
	return n
}

func calcPerShareEdge(buyPrice, sellPrice float64, buyFeeBps, sellFeeBps int64) (gross, net, netBps float64, ok bool) {
	buyMicros, okBuy := priceToMicros(buyPrice)
	sellMicros, okSell := priceToMicros(sellPrice)
	if !okBuy || !okSell {
		return 0, 0, 0, false
	}

	buyCostMicros := applyBpsRounded(buyMicros, bpsScale+buyFeeBps)
	sellProceedsMicros := applyBpsRounded(sellMicros, bpsScale-sellFeeBps)
	if buyCostMicros <= 0 {
		return 0, 0, 0, false
	}

	grossMicros := sellMicros - buyMicros
	netMicros := sellProceedsMicros - buyCostMicros

	gross = microsToFloat(grossMicros)
	net = microsToFloat(netMicros)
	netBps = float64(netMicros) * float64(bpsScale) / float64(buyCostMicros)
	return gross, net, netBps, true
}

func calcTradeNet(sellUSD, buyUSD float64, buyFeeBps, sellFeeBps int64) (netUSD, netBps float64, ok bool) {
	sellMicros, okSell := amountToMicros(sellUSD)
	buyMicros, okBuy := amountToMicros(buyUSD)
	if !okSell || !okBuy || buyMicros <= 0 {
		return 0, 0, false
	}

	buyCostMicros := applyBpsRounded(buyMicros, bpsScale+buyFeeBps)
	sellProceedsMicros := applyBpsRounded(sellMicros, bpsScale-sellFeeBps)
	if buyCostMicros <= 0 {
		return 0, 0, false
	}

	netMicros := sellProceedsMicros - buyCostMicros
	netUSD = microsToFloat(netMicros)
	netBps = float64(netMicros) * float64(bpsScale) / float64(buyCostMicros)
	return netUSD, netBps, true
}

func priceToMicros(v float64) (int64, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 1 {
		return 0, false
	}
	return int64(math.Round(v * float64(priceScale))), true
}

func amountToMicros(v float64) (int64, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, false
	}
	if v > float64(math.MaxInt64)/(float64(priceScale)*float64(bpsScale)) {
		return 0, false
	}
	return int64(math.Round(v * float64(priceScale))), true
}

func applyBpsRounded(value, bps int64) int64 {
	if value == 0 || bps == 0 {
		return 0
	}
	numerator := value * bps
	if numerator >= 0 {
		return (numerator + bpsScale/2) / bpsScale
	}
	return (numerator - bpsScale/2) / bpsScale
}

func microsToFloat(v int64) float64 {
	return float64(v) / float64(priceScale)
}

func envFloat(name string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func printOpp(o ArbOpportunity, rank int, green, yellow, red, cyan, reset, bold, dim string, minNetBps float64) {
	color := green
	if o.NetBps < 0 {
		color = red
	} else if o.NetBps < minNetBps {
		color = yellow
	}

	minLiq := math.Min(o.PredictLiq, o.PolyLiq)

	fmt.Printf("%s#%d %s [%s]%s\n", bold, rank, o.Type, o.PairID, reset)
	fmt.Printf("  %s\"%s\"%s\n", dim, o.Question, reset)
	fmt.Printf("  Net/share: %s$%+.4f%s  Gross/share: $%+.4f  |  ROI: %s%+.0f bps%s\n",
		color, o.NetProfit, reset, o.GrossProfit, color, o.NetBps, reset)
	fmt.Printf("  BUY  %-25s @ %.4f  [%s]\n", o.BuyPlatform, o.BuyPrice, o.BuyPriceSrc)
	fmt.Printf("  SELL %-25s @ %.4f  [%s]\n", o.SellPlatform, o.SellPrice, o.SellPriceSrc)
	fmt.Printf("  Liquidity: Predict $%s | Poly $%s | Min $%s\n",
		fmtUSD(o.PredictLiq), fmtUSD(o.PolyLiq), fmtUSD(minLiq))

	if o.QuoteSkewMs >= 0 {
		fmt.Printf("  Quote skew: %ds between legs\n", o.QuoteSkewMs/1000)
	}
	if !o.Executable {
		reason := o.NotExecReason
		if reason == "" {
			reason = "not executable"
		}
		fmt.Printf("  %sUNEXECUTABLE:%s %s\n", red, reset, reason)
	}

	if o.BuyDepthUSD > 0 || o.SellDepthUSD > 0 {
		fmt.Printf("  %sDepth:%s Buy $%s (%d lvls) | Sell $%s (%d lvls) | MaxTrade $%s\n",
			cyan, reset,
			fmtUSD(o.BuyDepthUSD), o.BuyLevels,
			fmtUSD(o.SellDepthUSD), o.SellLevels,
			fmtUSD(o.MaxTradeUSD))
	}

	if len(o.Fills) > 0 {
		hasFeasible := false
		for _, f := range o.Fills {
			if f.Feasible {
				hasFeasible = true
				break
			}
		}
		if hasFeasible {
			fmt.Printf("  %sFill Sim:%s\n", cyan, reset)
			fmt.Printf("    %-8s %-9s %-9s %-10s %-10s %s\n",
				"Size", "BuyVWAP", "SellVWAP", "Gross$", "Net$", "Bps")
			for _, f := range o.Fills {
				if !f.Feasible {
					fmt.Printf("    $%-7s %s(no depth)%s\n", fmtUSD(f.SizeUSD), dim, reset)
					continue
				}
				fColor := green
				if f.NetBps < 0 {
					fColor = red
				} else if f.NetBps < minNetBps {
					fColor = yellow
				}
				fmt.Printf("    $%-7s %-9.4f %-9.4f %-10.2f %s%-10.2f%s %s%+.0f%s\n",
					fmtUSD(f.SizeUSD), f.BuyVWAP, f.SellVWAP,
					f.GrossProfit, fColor, f.NetProfit, reset, fColor, f.NetBps, reset)
			}
		}
	}
	fmt.Println()
}

// resolvePrice picks the first non-zero value with source tracking.
// Does NOT fall back to mid/price — caller must handle that separately.
func resolvePrice(v1 float64, src1 string, v2 float64, src2 string) resolvedPrice {
	if v1 > 0 {
		return resolvedPrice{v1, src1}
	}
	if v2 > 0 {
		return resolvedPrice{v2, src2}
	}
	return resolvedPrice{0, ""}
}

// derefOB safely extracts bestBid (isBid=true) or bestAsk (isBid=false) from orderbook.
func derefOB(ob *market.OrderbookView, isBid bool) float64 {
	if ob == nil {
		return 0
	}
	if isBid {
		return deref(ob.BestBid)
	}
	return deref(ob.BestAsk)
}

// simulateBuy walks the asks to calculate VWAP for buying a given USD amount of shares.
func simulateBuy(asks []market.OrderbookRow, usd float64) FillResult {
	if len(asks) == 0 {
		return FillResult{}
	}

	var totalShares, totalCost float64
	levels := 0
	bestPrice := asks[0].Price

	for _, lvl := range asks {
		if lvl.Price <= 0 || lvl.Size <= 0 {
			continue
		}
		levels++
		lvlCost := lvl.Price * lvl.Size
		remaining := usd - totalCost
		if remaining <= 0 {
			break
		}
		if lvlCost >= remaining {
			shares := remaining / lvl.Price
			totalShares += shares
			totalCost += remaining
			break
		}
		totalShares += lvl.Size
		totalCost += lvlCost
	}

	var maxFill float64
	for _, lvl := range asks {
		if lvl.Price > 0 && lvl.Size > 0 {
			maxFill += lvl.Price * lvl.Size
		}
	}

	result := FillResult{
		FilledShares: totalShares,
		FilledUSD:    totalCost,
		MaxFillUSD:   maxFill,
		Levels:       levels,
	}
	if totalShares > 0 {
		result.VWAP = totalCost / totalShares
		if bestPrice > 0 {
			result.Slippage = result.VWAP/bestPrice - 1.0
		}
	}
	return result
}

// simulateSell walks the bids to calculate VWAP for selling a given number of shares.
func simulateSell(bids []market.OrderbookRow, shares float64) FillResult {
	if len(bids) == 0 {
		return FillResult{}
	}

	var totalShares, totalRevenue float64
	levels := 0
	bestPrice := bids[0].Price

	for _, lvl := range bids {
		if lvl.Price <= 0 || lvl.Size <= 0 {
			continue
		}
		levels++
		remaining := shares - totalShares
		if remaining <= 0 {
			break
		}
		if lvl.Size >= remaining {
			totalRevenue += remaining * lvl.Price
			totalShares += remaining
			break
		}
		totalShares += lvl.Size
		totalRevenue += lvl.Size * lvl.Price
	}

	var maxFill float64
	for _, lvl := range bids {
		if lvl.Price > 0 && lvl.Size > 0 {
			maxFill += lvl.Price * lvl.Size
		}
	}

	result := FillResult{
		FilledShares: totalShares,
		FilledUSD:    totalRevenue,
		MaxFillUSD:   maxFill,
		Levels:       levels,
	}
	if totalShares > 0 {
		result.VWAP = totalRevenue / totalShares
		if bestPrice > 0 {
			result.Slippage = 1.0 - result.VWAP/bestPrice
		}
	}
	return result
}

// fillDepth populates depth info and simulated fills for a cross arb.
func fillDepth(
	opp *ArbOpportunity,
	buyAsks, sellBids []market.OrderbookRow,
	buyFeeBps, sellFeeBps int64,
	sizes []float64,
	minFillRatio float64,
) {
	for _, lvl := range buyAsks {
		if lvl.Price > 0 && lvl.Size > 0 {
			opp.BuyDepthUSD += lvl.Price * lvl.Size
		}
	}
	opp.BuyLevels = len(buyAsks)

	for _, lvl := range sellBids {
		if lvl.Price > 0 && lvl.Size > 0 {
			opp.SellDepthUSD += lvl.Price * lvl.Size
		}
	}
	opp.SellLevels = len(sellBids)
	opp.MaxTradeUSD = math.Min(opp.BuyDepthUSD, opp.SellDepthUSD)

	if len(buyAsks) == 0 && len(sellBids) == 0 {
		return
	}

	for _, size := range sizes {
		sf := SimFill{SizeUSD: size}

		buyResult := simulateBuy(buyAsks, size)
		if buyResult.FilledShares == 0 || buyResult.FilledUSD < size*minFillRatio {
			sf.Feasible = false
			opp.Fills = append(opp.Fills, sf)
			continue
		}

		sellResult := simulateSell(sellBids, buyResult.FilledShares)
		if sellResult.FilledShares < buyResult.FilledShares*minFillRatio {
			sf.Feasible = false
			opp.Fills = append(opp.Fills, sf)
			continue
		}

		sf.Feasible = true
		sf.BuyVWAP = buyResult.VWAP
		sf.SellVWAP = sellResult.VWAP
		sf.GrossProfit = sellResult.FilledUSD - buyResult.FilledUSD
		netUSD, netBps, ok := calcTradeNet(sellResult.FilledUSD, buyResult.FilledUSD, buyFeeBps, sellFeeBps)
		if !ok {
			sf.Feasible = false
			opp.Fills = append(opp.Fills, sf)
			continue
		}
		sf.NetProfit = netUSD
		sf.NetBps = netBps
		opp.Fills = append(opp.Fills, sf)
	}
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func fmtUSD(v float64) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("%.1fM", v/1_000_000)
	}
	if v >= 1_000 {
		return fmt.Sprintf("%.1fK", v/1_000)
	}
	return fmt.Sprintf("%.0f", v)
}
