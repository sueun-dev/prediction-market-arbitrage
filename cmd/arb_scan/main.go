package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"predict-market/internal/market"
)

const (
	predictTakerFeeBps    = 200.0
	polymarketTakerFeeBps = 100.0
)

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
	BuyPriceSrc  string // "ob" = orderbook bid/ask, "price" = mid price, "pricing" = pricing field
	SellPriceSrc string

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

	// Fee multipliers: when you BUY, you pay more; when you SELL, you receive less.
	predictBuyMul := 1.0 + predictTakerFeeBps/10000.0   // 1.02
	predictSellMul := 1.0 - predictTakerFeeBps/10000.0   // 0.98
	polyBuyMul := 1.0 + polymarketTakerFeeBps/10000.0     // 1.01
	polySellMul := 1.0 - polymarketTakerFeeBps/10000.0    // 0.99

	simSizes := []float64{100, 500, 1000, 5000}

	var opps []ArbOpportunity
	skippedMid := 0

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
		if pAsk.value > 0 && mBid.value > 0 {
			// Reject if either side is a mid price — can't execute at mid
			if pAsk.source == "mid" || mBid.source == "mid" {
				skippedMid++
			} else {
				gross := mBid.value - pAsk.value
				net := mBid.value*polySellMul - pAsk.value*predictBuyMul
				if net > -0.03 {
					opp := ArbOpportunity{
						PairID: pair.ID, Question: pair.Question,
						Type: "YES_CROSS", BuyPlatform: "Predict", BuyPrice: pAsk.value,
						SellPlatform: "Polymarket", SellPrice: mBid.value,
						GrossProfit: gross, NetProfit: net, NetBps: net * 10000,
						PredictLiq: pLiq, PolyLiq: mLiq,
						BuyPriceSrc: pAsk.source, SellPriceSrc: mBid.source,
					}
					fillDepth(&opp, pAsks, mBids, predictBuyMul, polySellMul, simSizes)
					opps = append(opps, opp)
				}
			}
		}

		// === YES Cross Direction B: Buy on Poly (at ask), Sell on Predict (at bid) ===
		if mAsk.value > 0 && pBid.value > 0 {
			if mAsk.source == "mid" || pBid.source == "mid" {
				skippedMid++
			} else {
				gross := pBid.value - mAsk.value
				net := pBid.value*predictSellMul - mAsk.value*polyBuyMul
				if net > -0.03 {
					opp := ArbOpportunity{
						PairID: pair.ID, Question: pair.Question,
						Type: "YES_CROSS", BuyPlatform: "Polymarket", BuyPrice: mAsk.value,
						SellPlatform: "Predict", SellPrice: pBid.value,
						GrossProfit: gross, NetProfit: net, NetBps: net * 10000,
						PredictLiq: pLiq, PolyLiq: mLiq,
						BuyPriceSrc: mAsk.source, SellPriceSrc: pBid.source,
					}
					fillDepth(&opp, mAsks, pBids, polyBuyMul, predictSellMul, simSizes)
					opps = append(opps, opp)
				}
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

	// Sort by net profit descending
	sort.Slice(opps, func(i, j int) bool {
		return opps[i].NetProfit > opps[j].NetProfit
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
		if o.NetProfit > 0 {
			profitable++
		}
	}

	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n", bold, reset)
	fmt.Printf("%s  ARB SCAN — YES CROSS ONLY (strict pricing)%s\n", bold, reset)
	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n\n", bold, reset)
	fmt.Printf("Pairs scanned:            %d\n", payload.Count)
	fmt.Printf("Total arb checks:         %d\n", len(opps))
	fmt.Printf("Skipped (mid-price only): %d\n", skippedMid)
	fmt.Printf("Profitable (net > 0 bps): %s%d%s\n", green, profitable, reset)
	fmt.Printf("Fee model:                Predict %.0f bps | Polymarket %.0f bps\n", predictTakerFeeBps, polymarketTakerFeeBps)
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
			printOpp(o, shown+1, green, yellow, red, cyan, reset, bold, dim)
			shown++
		}
	} else {
		fmt.Printf("%s── PROFITABLE (%d) ──%s\n\n", green, profitable, reset)
		rank := 0
		for _, o := range opps {
			if o.NetProfit <= 0 {
				break
			}
			rank++
			printOpp(o, rank, green, yellow, red, cyan, reset, bold, dim)
		}
	}

	// Summary
	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n", bold, reset)
	fmt.Printf("%s  SUMMARY%s\n", bold, reset)
	fmt.Printf("%s══════════════════════════════════════════════════════════════%s\n\n", bold, reset)

	if len(opps) > 0 && opps[0].NetProfit > 0 {
		fmt.Printf("Best net profit:  %s+%.2f%% (%.0f bps)%s\n", green, opps[0].NetProfit*100, opps[0].NetBps, reset)
	} else if len(opps) > 0 {
		fmt.Printf("Best net profit:  %s%.2f%% (%.0f bps)%s  (not profitable)\n", red, opps[0].NetProfit*100, opps[0].NetBps, reset)
	}
	fmt.Printf("Profitable:       %d / %d\n", profitable, len(opps))

	// Data quality note
	fmt.Printf("\n%sData quality notes:%s\n", bold, reset)
	obCount := 0
	pricingCount := 0
	for _, o := range opps {
		if o.NetProfit <= 0 {
			break
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
	fmt.Printf("  Skipped due to mid-price only:      %d\n", skippedMid)
	fmt.Printf("  Polymarket depth (asks/bids):       empty (bestBid/bestAsk only)\n")
	fmt.Printf("  Predict depth:                      full orderbook available\n")
}

func printOpp(o ArbOpportunity, rank int, green, yellow, red, cyan, reset, bold, dim string) {
	color := green
	if o.NetProfit <= 0 {
		color = red
	} else if o.NetBps < 50 {
		color = yellow
	}

	netPct := o.NetProfit * 100
	grossPct := o.GrossProfit * 100
	minLiq := math.Min(o.PredictLiq, o.PolyLiq)

	fmt.Printf("%s#%d %s [%s]%s\n", bold, rank, o.Type, o.PairID, reset)
	fmt.Printf("  %s\"%s\"%s\n", dim, o.Question, reset)
	fmt.Printf("  Net: %s%s%+.2f%%%s  Gross: %+.2f%%  |  %s%+.0f bps%s\n",
		bold, color, netPct, reset, grossPct, color, o.NetBps, reset)
	fmt.Printf("  BUY  %-25s @ %.4f  [%s]\n", o.BuyPlatform, o.BuyPrice, o.BuyPriceSrc)
	fmt.Printf("  SELL %-25s @ %.4f  [%s]\n", o.SellPlatform, o.SellPrice, o.SellPriceSrc)
	fmt.Printf("  Liquidity: Predict $%s | Poly $%s | Min $%s\n",
		fmtUSD(o.PredictLiq), fmtUSD(o.PolyLiq), fmtUSD(minLiq))

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
				} else if f.NetBps < 50 {
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
func fillDepth(opp *ArbOpportunity, buyAsks, sellBids []market.OrderbookRow, buyFeeMul, sellFeeMul float64, sizes []float64) {
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
		if buyResult.FilledShares == 0 || buyResult.FilledUSD < size*0.9 {
			sf.Feasible = false
			opp.Fills = append(opp.Fills, sf)
			continue
		}

		sellResult := simulateSell(sellBids, buyResult.FilledShares)
		if sellResult.FilledShares < buyResult.FilledShares*0.9 {
			sf.Feasible = false
			opp.Fills = append(opp.Fills, sf)
			continue
		}

		sf.Feasible = true
		sf.BuyVWAP = buyResult.VWAP
		sf.SellVWAP = sellResult.VWAP
		sf.GrossProfit = sellResult.FilledUSD - buyResult.FilledUSD
		sf.NetProfit = sellResult.FilledUSD*sellFeeMul - buyResult.FilledUSD*buyFeeMul
		if buyResult.FilledUSD > 0 {
			sf.NetBps = (sf.NetProfit / buyResult.FilledUSD) * 10000
		}
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

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
