package market

import "encoding/json"

// MarketStatistics holds aggregate trading stats for a market.
type MarketStatistics struct {
	TotalLiquidityUsd        float64  `json:"totalLiquidityUsd"`
	VolumeTotalUsd           float64  `json:"volumeTotalUsd"`
	Volume24hUsd             float64  `json:"volume24hUsd"`
	Volume24hChangeUsd       float64  `json:"volume24hChangeUsd"`
	PercentageChanceChange24h *float64 `json:"percentageChanceChange24h"`
}

// Tag represents a category tag.
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MarketCategory holds category metadata for a market.
type MarketCategory struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Description    *string `json:"description"`
	ImageUrl       string  `json:"imageUrl"`
	IsNegRisk      bool    `json:"isNegRisk"`
	IsYieldBearing bool    `json:"isYieldBearing"`
	StartsAt       *string `json:"startsAt"`
	EndsAt         *string `json:"endsAt"`
	Status         string  `json:"status"`
	HoldersCount   *string `json:"holdersCount"`
	CommentsTotal  *int    `json:"commentsTotal"`
	Tags           []Tag   `json:"tags"`
}

// OrderbookRow is a single price/size level.
type OrderbookRow struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

// LastOrderSettled from orderbook snapshot.
type LastOrderSettled struct {
	ID       string `json:"id,omitempty"`
	Price    string `json:"price"`
	Kind     string `json:"kind,omitempty"`
	Side     string `json:"side"`
	Outcome  string `json:"outcome,omitempty"`
	MarketID int    `json:"marketId,omitempty"`
}

// OrderbookView is a normalized orderbook representation.
type OrderbookView struct {
	UpdateTimestampMs *int64            `json:"updateTimestampMs"`
	OrderCount        *int              `json:"orderCount"`
	LastOrderSettled  *LastOrderSettled  `json:"lastOrderSettled"`
	BestAsk           *float64          `json:"bestAsk"`
	BestBid           *float64          `json:"bestBid"`
	Spread            *float64          `json:"spread"`
	SpreadCents       *float64          `json:"spreadCents"`
	Asks              []OrderbookRow    `json:"asks"`
	Bids              []OrderbookRow    `json:"bids"`
	SettlementsPending json.RawMessage  `json:"settlementsPending"`
}

// MarketOutcome represents an outcome of a market.
type MarketOutcome struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Index              int      `json:"index"`
	Status             *string  `json:"status"`
	OnChainID          *string  `json:"onChainId"`
	BidPriceInCurrency *float64 `json:"bidPriceInCurrency"`
	AskPriceInCurrency *float64 `json:"askPriceInCurrency"`
	SharesCount        *float64 `json:"sharesCount"`
	PositionsValueUsd  *float64 `json:"positionsValueUsd"`
	PositionsCount     int      `json:"positionsCount"`
	Price              *float64 `json:"price,omitempty"`
	BidPrice           *float64 `json:"bidPrice,omitempty"`
	AskPrice           *float64 `json:"askPrice,omitempty"`
}

// HolderPosition is a single holder's position.
type HolderPosition struct {
	ID            string          `json:"id"`
	Shares        string          `json:"shares"`
	ValueUsd      *float64        `json:"valueUsd"`
	SharesDecimal string          `json:"sharesDecimal"`
	Account       HolderAccount   `json:"account"`
}

// HolderAccount holds account info for a holder.
type HolderAccount struct {
	Address string  `json:"address"`
	Name    *string `json:"name"`
}

// OutcomeHolders holds holder data for a single outcome.
type OutcomeHolders struct {
	OutcomeID  string           `json:"outcomeId"`
	Index      int              `json:"index"`
	Name       string           `json:"name"`
	TotalCount int              `json:"totalCount"`
	Positions  []HolderPosition `json:"positions"`
}

// MarketHolders is the full holders data for a market.
type MarketHolders struct {
	Outcomes []OutcomeHolders `json:"outcomes"`
}

// CommentAccount holds account info in a comment.
type CommentAccount struct {
	Address  string  `json:"address"`
	Name     *string `json:"name"`
	ImageUrl *string `json:"imageUrl"`
}

// CommentNode is a single comment.
type CommentNode struct {
	ID             string          `json:"id"`
	Content        string          `json:"content"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      *string         `json:"updatedAt"`
	LikeCount      int             `json:"likeCount"`
	IsLikedByUser  bool            `json:"isLikedByUser"`
	ReplyCount     int             `json:"replyCount"`
	ReportCount    int             `json:"reportCount"`
	Account        CommentAccount  `json:"account"`
	ParentComment  *CommentRef     `json:"parentComment"`
	ReplyToComment *ReplyToRef     `json:"replyToComment"`
	Replies        *CommentReplies `json:"replies,omitempty"`
}

// CommentRef is a reference to a parent comment.
type CommentRef struct {
	ID string `json:"id"`
}

// ReplyToRef is a reference to a comment being replied to.
type ReplyToRef struct {
	ID      string         `json:"id"`
	Account ReplyToAccount `json:"account"`
}

// ReplyToAccount holds account info for a reply target.
type ReplyToAccount struct {
	Name    *string `json:"name"`
	Address string  `json:"address"`
}

// CommentReplies holds paginated replies.
type CommentReplies struct {
	PageInfo   CommentPageInfo  `json:"pageInfo"`
	TotalCount *int             `json:"totalCount"`
	Edges      []CommentReplyEdge `json:"edges"`
}

// CommentPageInfo for reply pagination.
type CommentPageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

// CommentReplyEdge wraps a reply node.
type CommentReplyEdge struct {
	Node CommentNode `json:"node"`
}

// CommentEdge wraps a comment node with cursor.
type CommentEdge struct {
	Cursor string      `json:"cursor"`
	Node   CommentNode `json:"node"`
}

// CategoryComments holds paginated comments for a category.
type CategoryComments struct {
	TotalCount int           `json:"totalCount"`
	Edges      []CommentEdge `json:"edges"`
}

// TimeseriesPoint is a single data point.
type TimeseriesPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// TimeseriesData holds timeseries for a market.
type TimeseriesData struct {
	DataGranularity string            `json:"dataGranularity"`
	Points          []TimeseriesPoint `json:"points"`
}

// StatusLogNode holds a status change event.
type StatusLogNode struct {
	Status          string  `json:"status"`
	Timestamp       string  `json:"timestamp"`
	TransactionHash *string `json:"transactionHash"`
}

// StatusLogEdge wraps a status log node.
type StatusLogEdge struct {
	Node StatusLogNode `json:"node"`
}

// StatusLogs holds paginated status logs.
type StatusLogs struct {
	Edges []StatusLogEdge `json:"edges"`
}

// BulletinBoardUpdate is a bulletin board entry.
type BulletinBoardUpdate struct {
	Content         string  `json:"content"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       *string `json:"updatedAt"`
	TransactionHash *string `json:"transactionHash"`
}

// MarketResolution holds resolution info.
type MarketResolution struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Index     int    `json:"index"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// OrderbookToken identifies a CLOB token for orderbook lookup.
type OrderbookToken struct {
	TokenID string `json:"tokenId"`
	Outcome string `json:"outcome"`
}

// NormalizedMarket is the unified market representation across platforms.
type NormalizedMarket struct {
	ID                      string                        `json:"id"`
	Title                   string                        `json:"title"`
	Question                string                        `json:"question"`
	Description             *string                       `json:"description"`
	ImageUrl                string                        `json:"imageUrl"`
	CreatedAt               *string                       `json:"createdAt"`
	Status                  string                        `json:"status"`
	IsTradingEnabled        bool                          `json:"isTradingEnabled"`
	ChancePercentage        *float64                      `json:"chancePercentage"`
	SpreadThreshold         string                        `json:"spreadThreshold"`
	SpreadThresholdDecimal  string                        `json:"spreadThresholdDecimal"`
	SpreadThresholdPercent  string                        `json:"spreadThresholdPercent"`
	ShareThreshold          string                        `json:"shareThreshold"`
	MakerFeeBps             int                           `json:"makerFeeBps"`
	TakerFeeBps             int                           `json:"takerFeeBps"`
	DecimalPrecision        int                           `json:"decimalPrecision"`
	OracleQuestionID        *string                       `json:"oracleQuestionId"`
	OracleTxHash            *string                       `json:"oracleTxHash"`
	ConditionID             *string                       `json:"conditionId"`
	ResolverAddress         *string                       `json:"resolverAddress"`
	QuestionIndex           *int                          `json:"questionIndex"`
	Category                MarketCategory                `json:"category"`
	Statistics              MarketStatistics              `json:"statistics"`
	Outcomes                []MarketOutcome               `json:"outcomes"`
	TotalPositions          int                           `json:"totalPositions"`
	Resolution              *MarketResolution             `json:"resolution"`
	StatusLogs              *StatusLogs                   `json:"statusLogs"`
	BulletinBoardUpdates    []BulletinBoardUpdate         `json:"bulletinBoardUpdates"`
	Orderbook               *OrderbookView                `json:"orderbook"`
	Holders                 *MarketHolders                `json:"holders"`
	Comments                *CategoryComments             `json:"comments"`
	Timeseries              map[string]*TimeseriesData    `json:"timeseries"`
	Source                  string                        `json:"source,omitempty"`
	SourceUrl               string                        `json:"sourceUrl,omitempty"`
	OrderbookTokens         []OrderbookToken              `json:"orderbookTokens,omitempty"`
}

// ViewOutcome is a trimmed outcome for the view payload.
type ViewOutcome struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Index          int    `json:"index"`
	PositionsCount int    `json:"positionsCount"`
}

// ViewMarket is a trimmed market for the view payload.
type ViewMarket struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Question         string           `json:"question"`
	ImageUrl         string           `json:"imageUrl"`
	Category         MarketCategory   `json:"category"`
	ChancePercentage *float64         `json:"chancePercentage"`
	SpreadThreshold  string           `json:"spreadThreshold"`
	SpreadDecimal    *string          `json:"spreadDecimal"`
	SpreadPercent    *string          `json:"spreadPercent"`
	MakerFeeBps      *int             `json:"makerFeeBps"`
	TakerFeeBps      *int             `json:"takerFeeBps"`
	IsTradingEnabled *bool            `json:"isTradingEnabled"`
	Status           *string          `json:"status"`
	ShareThreshold   *string          `json:"shareThreshold"`
	Statistics       *MarketStatistics `json:"statistics"`
	Outcomes         []ViewOutcome    `json:"outcomes"`
	TotalPositions   int              `json:"totalPositions"`
	Source           *string          `json:"source,omitempty"`
}

// OutcomePricing holds bid/ask/price for a single outcome.
type OutcomePricing struct {
	Bid   *float64 `json:"bid"`
	Ask   *float64 `json:"ask"`
	Price *float64 `json:"price"`
}

// PairPricing holds pricing for both platforms.
type PairPricing struct {
	Predict    YesNoPricing `json:"predict"`
	Polymarket YesNoPricing `json:"polymarket"`
}

// YesNoPricing holds yes/no pricing.
type YesNoPricing struct {
	Yes OutcomePricing `json:"yes"`
	No  OutcomePricing `json:"no"`
}

// TrimmedPredictMarket is a trimmed Predict.Fun market for pairs.
type TrimmedPredictMarket struct {
	ID             string           `json:"id"`
	Question       string           `json:"question"`
	Category       MarketCategory   `json:"category"`
	Statistics     MarketStatistics `json:"statistics"`
	TotalPositions int              `json:"totalPositions"`
	Orderbook      *OrderbookView   `json:"orderbook"`
	Source         string           `json:"source"`
	SourceUrl      string           `json:"sourceUrl"`
}

// TrimmedPolymarketMarket is a trimmed Polymarket market for pairs.
type TrimmedPolymarketMarket struct {
	ID              string           `json:"id"`
	Question        string           `json:"question"`
	Category        MarketCategory   `json:"category"`
	Statistics      MarketStatistics `json:"statistics"`
	Orderbook       *OrderbookView   `json:"orderbook"`
	OrderbookTokens []OrderbookToken `json:"orderbookTokens,omitempty"`
	Source          string           `json:"source"`
	SourceUrl       string           `json:"sourceUrl"`
}

// MarketPair represents a matched pair between Predict.Fun and Polymarket.
type MarketPair struct {
	ID         string                  `json:"id"`
	Similarity float64                 `json:"similarity"`
	Question   string                  `json:"question"`
	Predict    TrimmedPredictMarket    `json:"predict"`
	Polymarket TrimmedPolymarketMarket `json:"polymarket"`
	Pricing    PairPricing             `json:"pricing"`
}

// FullPayload is the output of fetch_open_markets.
type FullPayload struct {
	GeneratedAt string             `json:"generatedAt"`
	Count       int                `json:"count"`
	Markets     []NormalizedMarket `json:"markets"`
}

// ViewPayload is the view output.
type ViewPayload struct {
	GeneratedAt string       `json:"generatedAt"`
	Count       int          `json:"count"`
	Markets     []ViewMarket `json:"markets"`
}

// PairsPayload is the pairs output.
type PairsPayload struct {
	GeneratedAt string       `json:"generatedAt"`
	Count       int          `json:"count"`
	Pairs       []MarketPair `json:"pairs"`
}

// CombinedPayload is the combined full output.
type CombinedPayload struct {
	GeneratedAt string             `json:"generatedAt"`
	Count       int                `json:"count"`
	Markets     []NormalizedMarket `json:"markets"`
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T {
	return &v
}
