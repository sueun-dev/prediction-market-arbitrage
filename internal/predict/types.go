package predict

import "encoding/json"

// MarketIndex is a lightweight market entry from the index query.
type MarketIndex struct {
	ID               string         `json:"id"`
	Status           string         `json:"status"`
	IsTradingEnabled bool           `json:"isTradingEnabled"`
	Category         CategoryRef    `json:"category"`
}

// CategoryRef is a lightweight category reference.
type CategoryRef struct {
	ID string `json:"id"`
}

// PageInfo holds pagination state.
type PageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	StartCursor *string `json:"startCursor"`
	EndCursor   *string `json:"endCursor"`
}

// MarketIndexResponse is the GraphQL response for market index.
type MarketIndexResponse struct {
	Data struct {
		Markets struct {
			PageInfo PageInfo `json:"pageInfo"`
			Edges    []struct {
				Node MarketIndex `json:"node"`
			} `json:"edges"`
		} `json:"markets"`
	} `json:"data"`
	Errors []GQLError `json:"errors,omitempty"`
}

// GQLError is a GraphQL error.
type GQLError struct {
	Message string `json:"message"`
}

// MarketDetailResponse is the GraphQL response for market detail.
type MarketDetailResponse struct {
	Data struct {
		Market MarketDetail `json:"market"`
	} `json:"data"`
	Errors []GQLError `json:"errors,omitempty"`
}

// MarketDetail is the full market detail from GraphQL.
type MarketDetail struct {
	ID                 string           `json:"id"`
	Title              string           `json:"title"`
	Question           string           `json:"question"`
	Description        *string          `json:"description"`
	ImageUrl           string           `json:"imageUrl"`
	CreatedAt          string           `json:"createdAt"`
	Status             string           `json:"status"`
	IsTradingEnabled   bool             `json:"isTradingEnabled"`
	ChancePercentage   float64          `json:"chancePercentage"`
	SpreadThreshold    string           `json:"spreadThreshold"`
	ShareThreshold     string           `json:"shareThreshold"`
	MakerFeeBps        int              `json:"makerFeeBps"`
	TakerFeeBps        int              `json:"takerFeeBps"`
	DecimalPrecision   int              `json:"decimalPrecision"`
	OracleQuestionID   *string          `json:"oracleQuestionId"`
	OracleTxHash       *string          `json:"oracleTxHash"`
	ConditionID        *string          `json:"conditionId"`
	ResolverAddress    *string          `json:"resolverAddress"`
	QuestionIndex      *int             `json:"questionIndex"`
	Category           DetailCategory   `json:"category"`
	Statistics         DetailStatistics `json:"statistics"`
	Outcomes           DetailOutcomes   `json:"outcomes"`
	Resolution         *DetailResolution `json:"resolution"`
	StatusLogs         DetailStatusLogs `json:"statusLogs"`
	BulletinBoardUpdates []DetailBulletin `json:"bulletinBoardUpdates"`
}

// DetailCategory is the category from market detail.
type DetailCategory struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Description    *string         `json:"description"`
	ImageUrl       string          `json:"imageUrl"`
	IsNegRisk      bool            `json:"isNegRisk"`
	IsYieldBearing bool            `json:"isYieldBearing"`
	StartsAt       *string         `json:"startsAt"`
	EndsAt         *string         `json:"endsAt"`
	Status         string          `json:"status"`
	HoldersCount   *json.Number    `json:"holdersCount"`
	Comments       *DetailCommentCount `json:"comments"`
	Tags           *DetailTags     `json:"tags"`
}

// DetailCommentCount holds comment count.
type DetailCommentCount struct {
	TotalCount *int `json:"totalCount"`
}

// DetailTags holds tag edges.
type DetailTags struct {
	Edges []DetailTagEdge `json:"edges"`
}

// DetailTagEdge wraps a tag node.
type DetailTagEdge struct {
	Node DetailTag `json:"node"`
}

// DetailTag is a tag.
type DetailTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DetailStatistics holds market statistics.
type DetailStatistics struct {
	TotalLiquidityUsd         float64  `json:"totalLiquidityUsd"`
	VolumeTotalUsd            float64  `json:"volumeTotalUsd"`
	Volume24hUsd              float64  `json:"volume24hUsd"`
	Volume24hChangeUsd        float64  `json:"volume24hChangeUsd"`
	PercentageChanceChange24h *float64 `json:"percentageChanceChange24h"`
}

// DetailOutcomes wraps outcome edges.
type DetailOutcomes struct {
	Edges []DetailOutcomeEdge `json:"edges"`
}

// DetailOutcomeEdge wraps an outcome node.
type DetailOutcomeEdge struct {
	Node DetailOutcome `json:"node"`
}

// DetailOutcome is a market outcome.
type DetailOutcome struct {
	ID                 string            `json:"id"`
	Index              int               `json:"index"`
	Name               string            `json:"name"`
	Status             *string           `json:"status"`
	OnChainID          string            `json:"onChainId"`
	BidPriceInCurrency *float64          `json:"bidPriceInCurrency"`
	AskPriceInCurrency *float64          `json:"askPriceInCurrency"`
	Statistics         *OutcomeStats     `json:"statistics"`
	Positions          *OutcomePositions `json:"positions"`
}

// OutcomeStats holds outcome statistics.
type OutcomeStats struct {
	SharesCount      float64 `json:"sharesCount"`
	PositionsValueUsd float64 `json:"positionsValueUsd"`
}

// OutcomePositions holds position count.
type OutcomePositions struct {
	TotalCount int `json:"totalCount"`
}

// DetailResolution holds resolution data.
type DetailResolution struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Index     int    `json:"index"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// DetailStatusLogs wraps status log edges.
type DetailStatusLogs struct {
	Edges []DetailStatusLogEdge `json:"edges"`
}

// DetailStatusLogEdge wraps a status log node.
type DetailStatusLogEdge struct {
	Node DetailStatusLog `json:"node"`
}

// DetailStatusLog is a single status change.
type DetailStatusLog struct {
	Status          string  `json:"status"`
	Timestamp       string  `json:"timestamp"`
	TransactionHash *string `json:"transactionHash"`
}

// DetailBulletin is a bulletin board entry.
type DetailBulletin struct {
	Content         string  `json:"content"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       *string `json:"updatedAt"`
	TransactionHash *string `json:"transactionHash"`
}

// MarketHoldersResponse is the GraphQL response for market holders.
type MarketHoldersResponse struct {
	Data struct {
		Market struct {
			Outcomes struct {
				Edges []HolderOutcomeEdge `json:"edges"`
			} `json:"outcomes"`
		} `json:"market"`
	} `json:"data"`
	Errors []GQLError `json:"errors,omitempty"`
}

// HolderOutcomeEdge wraps an outcome node with positions.
type HolderOutcomeEdge struct {
	Node HolderOutcome `json:"node"`
}

// HolderOutcome is an outcome with holder positions.
type HolderOutcome struct {
	ID        string          `json:"id"`
	Index     int             `json:"index"`
	Name      string          `json:"name"`
	Positions HolderPositions `json:"positions"`
}

// HolderPositions holds paginated positions.
type HolderPositions struct {
	PageInfo   PageInfo          `json:"pageInfo"`
	TotalCount int               `json:"totalCount"`
	Edges      []HolderPosEdge   `json:"edges"`
}

// HolderPosEdge wraps a holder position.
type HolderPosEdge struct {
	Node   HolderPos `json:"node"`
	Cursor string    `json:"cursor"`
}

// HolderPos is a single holder position.
type HolderPos struct {
	ID       string       `json:"id"`
	Shares   string       `json:"shares"`
	ValueUsd *float64     `json:"valueUsd"`
	Account  HolderAcct   `json:"account"`
}

// HolderAcct holds holder account info.
type HolderAcct struct {
	Address string  `json:"address"`
	Name    *string `json:"name"`
}

// CommentsResponse is the GraphQL response for comments.
type CommentsResponse struct {
	Data struct {
		Comments struct {
			PageInfo   PageInfo           `json:"pageInfo"`
			TotalCount int                `json:"totalCount"`
			Edges      []CommentEdgeGQL   `json:"edges"`
		} `json:"comments"`
	} `json:"data"`
	Errors []GQLError `json:"errors,omitempty"`
}

// CommentEdgeGQL is a comment edge from GraphQL.
type CommentEdgeGQL struct {
	Cursor string         `json:"cursor"`
	Node   CommentNodeGQL `json:"node"`
}

// CommentNodeGQL is a comment node from GraphQL.
type CommentNodeGQL struct {
	ID             string                `json:"id"`
	Content        string                `json:"content"`
	CreatedAt      string                `json:"createdAt"`
	UpdatedAt      *string               `json:"updatedAt"`
	LikeCount      int                   `json:"likeCount"`
	IsLikedByUser  bool                  `json:"isLikedByUser"`
	ReplyCount     int                   `json:"replyCount"`
	ReportCount    int                   `json:"reportCount"`
	Account        CommentAcctGQL        `json:"account"`
	ParentComment  *CommentRefGQL        `json:"parentComment"`
	ReplyToComment *ReplyToRefGQL        `json:"replyToComment"`
	Replies        *CommentRepliesGQL    `json:"replies,omitempty"`
}

// CommentAcctGQL is an account in a comment.
type CommentAcctGQL struct {
	Address  string  `json:"address"`
	Name     *string `json:"name"`
	ImageUrl *string `json:"imageUrl"`
}

// CommentRefGQL is a parent comment reference.
type CommentRefGQL struct {
	ID string `json:"id"`
}

// ReplyToRefGQL is a reply-to reference.
type ReplyToRefGQL struct {
	ID      string         `json:"id"`
	Account ReplyToAcctGQL `json:"account"`
}

// ReplyToAcctGQL is account info in a reply-to reference.
type ReplyToAcctGQL struct {
	Name    *string `json:"name"`
	Address string  `json:"address"`
}

// CommentRepliesGQL holds paginated replies.
type CommentRepliesGQL struct {
	PageInfo   CommentPageInfoGQL    `json:"pageInfo"`
	TotalCount *int                  `json:"totalCount"`
	Edges      []CommentReplyEdgeGQL `json:"edges"`
}

// CommentPageInfoGQL is page info for replies.
type CommentPageInfoGQL struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

// CommentReplyEdgeGQL wraps a reply node.
type CommentReplyEdgeGQL struct {
	Node CommentNodeGQL `json:"node"`
}

// RepliesResponse is the GraphQL response for loading more replies.
type RepliesResponse struct {
	Data struct {
		Comment struct {
			ID         string             `json:"id"`
			ReplyCount int                `json:"replyCount"`
			Replies    CommentRepliesGQL  `json:"replies"`
		} `json:"comment"`
	} `json:"data"`
	Errors []GQLError `json:"errors,omitempty"`
}

// TimeseriesResponse is the GraphQL response for timeseries.
type TimeseriesResponse struct {
	Data struct {
		Category struct {
			Timeseries struct {
				PageInfo PageInfo              `json:"pageInfo"`
				Edges    []TimeseriesEdgeGQL   `json:"edges"`
			} `json:"timeseries"`
		} `json:"category"`
	} `json:"data"`
	Errors []GQLError `json:"errors,omitempty"`
}

// TimeseriesEdgeGQL wraps a timeseries node.
type TimeseriesEdgeGQL struct {
	Node TimeseriesNodeGQL `json:"node"`
}

// TimeseriesNodeGQL is a timeseries entry.
type TimeseriesNodeGQL struct {
	DataGranularity string             `json:"dataGranularity"`
	Market          struct{ ID string `json:"id"` } `json:"market"`
	Data            TimeseriesDataGQL  `json:"data"`
}

// TimeseriesDataGQL holds timeseries data edges.
type TimeseriesDataGQL struct {
	Edges []TimeseriesPointEdge `json:"edges"`
}

// TimeseriesPointEdge wraps a timeseries point.
type TimeseriesPointEdge struct {
	Node TimeseriesPointGQL `json:"node"`
}

// TimeseriesPointGQL is a single data point.
type TimeseriesPointGQL struct {
	X json.Number `json:"x"`
	Y json.Number `json:"y"`
}

// GQLRequest is a GraphQL request payload.
type GQLRequest struct {
	OperationName string      `json:"operationName"`
	Variables     interface{} `json:"variables"`
	Query         string      `json:"query"`
}

// OrderbookSnapshot from the WebSocket.
type OrderbookSnapshot struct {
	Version            int              `json:"version"`
	MarketID           int              `json:"marketId"`
	UpdateTimestampMs  int64            `json:"updateTimestampMs"`
	LastOrderSettled   *LastOrderInfo   `json:"lastOrderSettled,omitempty"`
	OrderCount         int              `json:"orderCount"`
	Asks               [][2]float64     `json:"asks"`
	Bids               [][2]float64     `json:"bids"`
	SettlementsPending json.RawMessage  `json:"settlementsPending,omitempty"`
}

// LastOrderInfo from orderbook.
type LastOrderInfo struct {
	ID       string `json:"id"`
	Price    string `json:"price"`
	Kind     string `json:"kind"`
	Side     string `json:"side"`
	Outcome  string `json:"outcome"`
	MarketID int    `json:"marketId"`
}

// WsMessage is a WebSocket message from Predict.Fun.
type WsMessage struct {
	Type  string           `json:"type"`
	Topic string           `json:"topic,omitempty"`
	Data  *OrderbookSnapshot `json:"data,omitempty"`
}

// WsSubscribe is a WebSocket subscribe request.
type WsSubscribe struct {
	RequestID int      `json:"requestId"`
	Method    string   `json:"method"`
	Params    []string `json:"params"`
}
