package polymarket

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"predict-market/internal/market"
)

// NormalizeMarket converts a Polymarket raw market to a NormalizedMarket.
func NormalizeMarket(raw RawMarket) market.NormalizedMarket {
	outcomeNames := parseStringArray(raw.Outcomes)
	outcomePrices := parseNumberArray(raw.OutcomePrices)
	clobTokenIDs := parseStringArray(raw.ClobTokenIds)

	outcomes := make([]market.MarketOutcome, len(outcomeNames))
	for i, name := range outcomeNames {
		var price *float64
		if i < len(outcomePrices) && outcomePrices[i] != nil {
			price = outcomePrices[i]
		}
		var onChainID *string
		if i < len(clobTokenIDs) {
			onChainID = &clobTokenIDs[i]
		}
		outcomes[i] = market.MarketOutcome{
			ID:        fmt.Sprintf("poly_%s_%d", raw.ID, i),
			Name:      name,
			Index:     i,
			OnChainID: onChainID,
			Price:     price,
		}
	}

	bestAsk := jsonNumberToFloat(raw.BestAsk)
	bestBid := jsonNumberToFloat(raw.BestBid)

	var spread, spreadCents *float64
	if bestAsk != nil && bestBid != nil {
		s := roundTo(*bestAsk-*bestBid, 6)
		spread = &s
		sc := roundTo(s*100, 4)
		spreadCents = &sc
	}

	liquidity := coalesceFloat(raw.LiquidityNum, jsonNumberToFloat(raw.Liquidity))
	volumeTotal := coalesceFloat(raw.VolumeNum, jsonNumberToFloat(raw.Volume))
	volume24h := coalesceFloat(raw.Volume24hr, raw.Volume24hrClob)

	// Chance = max outcome price
	var chance *float64
	for _, p := range outcomePrices {
		if p != nil {
			pct := roundTo(*p*100, 2)
			if chance == nil || pct > *chance {
				chance = &pct
			}
		}
	}

	question := deref(raw.Question, "Polymarket Market")
	image := deref(raw.Image, deref(raw.Icon, ""))
	closed := raw.Closed != nil && *raw.Closed
	accepting := raw.AcceptingOrders != nil && *raw.AcceptingOrders
	active := raw.Active != nil && *raw.Active

	status := "OPEN"
	if closed {
		status = "CLOSED"
	}

	catStatus := "INACTIVE"
	if active {
		catStatus = "ACTIVE"
	}

	var spreadStr, spreadDecimalStr, spreadPercentStr string
	if raw.Spread != nil {
		spreadStr = strconv.FormatFloat(*raw.Spread, 'f', -1, 64)
		spreadDecimalStr = spreadStr
	} else {
		spreadStr = "0"
		spreadDecimalStr = "0"
	}
	if spreadCents != nil {
		spreadPercentStr = fmt.Sprintf("%.2f¢", *spreadCents)
	} else {
		spreadPercentStr = "-"
	}

	sourceUrl := "https://polymarket.com/"
	if raw.Slug != nil && *raw.Slug != "" {
		sourceUrl = fmt.Sprintf("https://polymarket.com/market/%s", *raw.Slug)
	}

	var updateTimestamp *int64
	if raw.UpdatedAt != nil {
		if ms, err := parseISOTime(*raw.UpdatedAt); err == nil {
			updateTimestamp = ms
		}
	}

	var lastOrderSettled *market.LastOrderSettled
	if raw.LastTradePrice != nil {
		lastOrderSettled = &market.LastOrderSettled{
			Price: strconv.FormatFloat(*raw.LastTradePrice, 'f', -1, 64),
			Side:  "",
		}
	}

	orderbookTokens := make([]market.OrderbookToken, len(clobTokenIDs))
	for i, tokenID := range clobTokenIDs {
		outcome := fmt.Sprintf("Outcome %d", i+1)
		if i < len(outcomeNames) {
			outcome = outcomeNames[i]
		}
		orderbookTokens[i] = market.OrderbookToken{
			TokenID: tokenID,
			Outcome: outcome,
		}
	}

	return market.NormalizedMarket{
		ID:                     fmt.Sprintf("poly_%s", raw.ID),
		Title:                  question,
		Question:               question,
		Description:            raw.Description,
		ImageUrl:               image,
		CreatedAt:              raw.CreatedAt,
		Status:                 status,
		IsTradingEnabled:       accepting,
		ChancePercentage:       chance,
		SpreadThreshold:        spreadStr,
		SpreadThresholdDecimal: spreadDecimalStr,
		SpreadThresholdPercent: spreadPercentStr,
		ShareThreshold:         "",
		MakerFeeBps:            0,
		TakerFeeBps:            0,
		DecimalPrecision:       6,
		ConditionID:            raw.ConditionId,
		Category: market.MarketCategory{
			ID:          deref(raw.Category, "polymarket"),
			Title:       deref(raw.Category, "Polymarket"),
			Description: raw.Description,
			ImageUrl:    image,
			IsNegRisk:   raw.NegRisk != nil && *raw.NegRisk,
			StartsAt:    coalesceString(raw.StartDateIso, raw.StartDate),
			EndsAt:      coalesceString(raw.EndDateIso, raw.EndDate),
			Status:      catStatus,
			Tags:        []market.Tag{},
		},
		Statistics: market.MarketStatistics{
			TotalLiquidityUsd: derefFloat(liquidity),
			VolumeTotalUsd:    derefFloat(volumeTotal),
			Volume24hUsd:      derefFloat(volume24h),
		},
		Outcomes:             outcomes,
		BulletinBoardUpdates: []market.BulletinBoardUpdate{},
		Orderbook: &market.OrderbookView{
			UpdateTimestampMs: updateTimestamp,
			LastOrderSettled:  lastOrderSettled,
			BestAsk:           bestAsk,
			BestBid:           bestBid,
			Spread:            spread,
			SpreadCents:       spreadCents,
			Asks:              []market.OrderbookRow{},
			Bids:              []market.OrderbookRow{},
		},
		Source:          "Polymarket",
		SourceUrl:       sourceUrl,
		OrderbookTokens: orderbookTokens,
	}
}

func parseStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try as []string first
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}

	// Try as a JSON string containing an array
	var s string
	if json.Unmarshal(raw, &s) == nil {
		var arr2 []string
		if json.Unmarshal([]byte(s), &arr2) == nil {
			return arr2
		}
	}

	return nil
}

func parseNumberArray(raw json.RawMessage) []*float64 {
	if len(raw) == 0 {
		return nil
	}

	// Try as array of mixed types
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		result := make([]*float64, len(arr))
		for i, item := range arr {
			result[i] = rawToFloat(item)
		}
		return result
	}

	// Try as string containing array
	var s string
	if json.Unmarshal(raw, &s) == nil {
		var arr2 []json.RawMessage
		if json.Unmarshal([]byte(s), &arr2) == nil {
			result := make([]*float64, len(arr2))
			for i, item := range arr2 {
				result[i] = rawToFloat(item)
			}
			return result
		}
	}

	return nil
}

func rawToFloat(raw json.RawMessage) *float64 {
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return &f
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return &v
		}
	}
	return nil
}

func jsonNumberToFloat(n *json.Number) *float64 {
	if n == nil {
		return nil
	}
	f, err := n.Float64()
	if err != nil {
		return nil
	}
	return &f
}

func roundTo(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}

func deref(p *string, fallback string) string {
	if p != nil {
		return *p
	}
	return fallback
}

func derefFloat(p *float64) float64 {
	if p != nil {
		return *p
	}
	return 0
}

func coalesceFloat(vals ...*float64) *float64 {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func coalesceString(vals ...*string) *string {
	for _, v := range vals {
		if v != nil && *v != "" {
			return v
		}
	}
	return nil
}

func parseISOTime(s string) (*int64, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			ms := t.UnixMilli()
			return &ms, nil
		}
	}
	return nil, fmt.Errorf("cannot parse time: %s", s)
}
