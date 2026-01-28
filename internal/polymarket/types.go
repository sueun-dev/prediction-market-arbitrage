package polymarket

import "encoding/json"

// RawMarket is a market from the Polymarket gamma API.
type RawMarket struct {
	ID               string          `json:"id"`
	Question         *string         `json:"question"`
	Description      *string         `json:"description"`
	Image            *string         `json:"image"`
	Icon             *string         `json:"icon"`
	CreatedAt        *string         `json:"createdAt"`
	Closed           *bool           `json:"closed"`
	AcceptingOrders  *bool           `json:"acceptingOrders"`
	Active           *bool           `json:"active"`
	Outcomes         json.RawMessage `json:"outcomes"`
	OutcomePrices    json.RawMessage `json:"outcomePrices"`
	ClobTokenIds     json.RawMessage `json:"clobTokenIds"`
	BestAsk          *json.Number    `json:"bestAsk"`
	BestBid          *json.Number    `json:"bestBid"`
	LiquidityNum     *float64        `json:"liquidityNum"`
	Liquidity        *json.Number    `json:"liquidity"`
	VolumeNum        *float64        `json:"volumeNum"`
	Volume           *json.Number    `json:"volume"`
	Volume24hr       *float64        `json:"volume24hr"`
	Volume24hrClob   *float64        `json:"volume24hrClob"`
	LastTradePrice   *float64        `json:"lastTradePrice"`
	Spread           *float64        `json:"spread"`
	Category         *string         `json:"category"`
	ConditionId      *string         `json:"conditionId"`
	NegRisk          *bool           `json:"negRisk"`
	StartDateIso     *string         `json:"startDateIso"`
	StartDate        *string         `json:"startDate"`
	EndDateIso       *string         `json:"endDateIso"`
	EndDate          *string         `json:"endDate"`
	Slug             *string         `json:"slug"`
	UpdatedAt        *string         `json:"updatedAt"`
}
