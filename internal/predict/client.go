package predict

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"predict-market/internal/config"
	"predict-market/internal/market"
)

// httpClient is a shared, optimized HTTP client for all GraphQL requests.
var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
		MaxIdleConns:         100,
		MaxIdleConnsPerHost:  100,
		MaxConnsPerHost:      100,
		IdleConnTimeout:      90 * time.Second,
		TLSHandshakeTimeout:  5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:    true,
		DisableCompression:   false,
	},
}

// PostGraphQL sends a GraphQL request and decodes the response.
// T should be a struct with Data and optional Errors fields.
func PostGraphQL[T any](ctx context.Context, payload interface{}, cfg config.Config) (T, error) {
	var zero T
	body, err := json.Marshal(payload)
	if err != nil {
		return zero, fmt.Errorf("marshal: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(time.Duration(cfg.BackoffSeconds*float64(attempt)*1000) * time.Millisecond):
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds*1000)*time.Millisecond)
		req, err := http.NewRequestWithContext(reqCtx, "POST", GraphQLURL, bytes.NewReader(body))
		if err != nil {
			cancel()
			return zero, fmt.Errorf("new request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if cfg.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
		}
		if cfg.Cookie != "" {
			req.Header.Set("Cookie", cfg.Cookie)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
			continue
		}

		// Single-pass: unmarshal into the full response type which contains both Data and Errors
		var result T
		if err := json.Unmarshal(respBody, &result); err != nil {
			return zero, fmt.Errorf("unmarshal: %w", err)
		}

		// Check for GraphQL errors via a lightweight re-parse
		var errCheck struct {
			Errors []GQLError `json:"errors"`
		}
		if json.Unmarshal(respBody, &errCheck) == nil && len(errCheck.Errors) > 0 {
			lastErr = fmt.Errorf("graphql: %s", errCheck.Errors[0].Message)
			continue
		}

		return result, nil
	}

	return zero, fmt.Errorf("all retries exhausted: %w", lastErr)
}

// FetchMarketIndex fetches all market index entries with pagination.
func FetchMarketIndex(ctx context.Context, cfg config.Config) ([]MarketIndex, error) {
	var markets []MarketIndex
	var cursor *string

	for {
		pagination := map[string]interface{}{"first": cfg.PageSize}
		if cursor != nil {
			pagination["after"] = *cursor
		}

		payload := GQLRequest{
			OperationName: "GetMarkets",
			Variables: map[string]interface{}{
				"filter":     map[string]interface{}{"isResolved": false},
				"sort":       cfg.Sort,
				"pagination": pagination,
			},
			Query: GetMarketsIndexQuery,
		}

		resp, err := PostGraphQL[MarketIndexResponse](ctx, payload, cfg)
		if err != nil {
			return nil, fmt.Errorf("fetch market index: %w", err)
		}

		for _, edge := range resp.Data.Markets.Edges {
			markets = append(markets, edge.Node)
		}

		if !resp.Data.Markets.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Markets.PageInfo.EndCursor

		if cfg.SleepSeconds > 0 {
			select {
			case <-ctx.Done():
				return markets, ctx.Err()
			case <-time.After(time.Duration(cfg.SleepSeconds*1000) * time.Millisecond):
			}
		}
	}

	return markets, nil
}

// StreamMarketIndex streams market index entries through a channel as pages arrive.
// The channel is closed when all pages are fetched or on error.
// Returns an error channel that will receive at most one error.
func StreamMarketIndex(ctx context.Context, cfg config.Config) (<-chan MarketIndex, <-chan error) {
	ch := make(chan MarketIndex, cfg.PageSize*2)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(errCh)

		var cursor *string
		for {
			pagination := map[string]interface{}{"first": cfg.PageSize}
			if cursor != nil {
				pagination["after"] = *cursor
			}

			payload := GQLRequest{
				OperationName: "GetMarkets",
				Variables: map[string]interface{}{
					"filter":     map[string]interface{}{"isResolved": false},
					"sort":       cfg.Sort,
					"pagination": pagination,
				},
				Query: GetMarketsIndexQuery,
			}

			resp, err := PostGraphQL[MarketIndexResponse](ctx, payload, cfg)
			if err != nil {
				errCh <- fmt.Errorf("fetch market index: %w", err)
				return
			}

			for _, edge := range resp.Data.Markets.Edges {
				select {
				case ch <- edge.Node:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}

			if !resp.Data.Markets.PageInfo.HasNextPage {
				return
			}
			cursor = resp.Data.Markets.PageInfo.EndCursor

			if cfg.SleepSeconds > 0 {
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				case <-time.After(time.Duration(cfg.SleepSeconds*1000) * time.Millisecond):
				}
			}
		}
	}()

	return ch, errCh
}

// FetchMarketDetail fetches full detail for a single market.
func FetchMarketDetail(ctx context.Context, cfg config.Config, marketID string) (MarketDetail, error) {
	payload := GQLRequest{
		OperationName: "GetMarketFull",
		Variables:     map[string]interface{}{"marketId": marketID},
		Query:         GetMarketDetailQuery,
	}

	resp, err := PostGraphQL[MarketDetailResponse](ctx, payload, cfg)
	if err != nil {
		return MarketDetail{}, fmt.Errorf("fetch detail %s: %w", marketID, err)
	}
	return resp.Data.Market, nil
}

// FetchOutcomeHolders fetches holders for a single outcome.
func FetchOutcomeHolders(ctx context.Context, cfg config.Config, marketID string, outcomeIndex int) (int, []HolderPos, error) {
	var positions []HolderPos
	var cursor *string
	totalCount := 0

	for {
		pagination := map[string]interface{}{"first": 50}
		if cursor != nil {
			pagination["after"] = *cursor
		}

		payload := GQLRequest{
			OperationName: "GetMarketHolders",
			Variables: map[string]interface{}{
				"marketId":   marketID,
				"filter":     map[string]interface{}{"outcomeIndex": outcomeIndex},
				"pagination": pagination,
			},
			Query: GetMarketHoldersQuery,
		}

		resp, err := PostGraphQL[MarketHoldersResponse](ctx, payload, cfg)
		if err != nil {
			return 0, nil, err
		}

		if len(resp.Data.Market.Outcomes.Edges) == 0 {
			break
		}

		outcome := resp.Data.Market.Outcomes.Edges[0].Node
		totalCount = outcome.Positions.TotalCount

		for _, edge := range outcome.Positions.Edges {
			positions = append(positions, edge.Node)
			if cfg.HoldersLimit > 0 && len(positions) >= cfg.HoldersLimit {
				return totalCount, positions, nil
			}
		}

		if !outcome.Positions.PageInfo.HasNextPage {
			break
		}
		cursor = outcome.Positions.PageInfo.EndCursor
	}

	return totalCount, positions, nil
}

// FetchMarketHolders fetches all holders for a market.
func FetchMarketHolders(ctx context.Context, cfg config.Config, detail MarketDetail) (*market.MarketHolders, error) {
	var outcomes []market.OutcomeHolders

	for _, edge := range detail.Outcomes.Edges {
		o := edge.Node
		totalCount, positions, err := FetchOutcomeHolders(ctx, cfg, detail.ID, o.Index)
		if err != nil {
			return nil, err
		}

		holders := make([]market.HolderPosition, len(positions))
		for i, pos := range positions {
			holders[i] = market.HolderPosition{
				ID:            pos.ID,
				Shares:        pos.Shares,
				ValueUsd:      pos.ValueUsd,
				SharesDecimal: market.FormatDecimal(pos.Shares, market.WeiDecimals, 6),
				Account: market.HolderAccount{
					Address: pos.Account.Address,
					Name:    pos.Account.Name,
				},
			}
		}

		outcomes = append(outcomes, market.OutcomeHolders{
			OutcomeID:  o.ID,
			Index:      o.Index,
			Name:       o.Name,
			TotalCount: totalCount,
			Positions:  holders,
		})
	}

	return &market.MarketHolders{Outcomes: outcomes}, nil
}

// FetchComments fetches comments for a category.
func FetchComments(ctx context.Context, cfg config.Config, categoryID string) (*market.CategoryComments, error) {
	var edges []market.CommentEdge
	var cursor *string
	totalCount := 0

	for {
		pagination := map[string]interface{}{"first": 30}
		if cursor != nil {
			pagination["after"] = *cursor
		}

		payload := GQLRequest{
			OperationName: "GetComments",
			Variables: map[string]interface{}{
				"categoryId":        categoryID,
				"onlyHolders":       false,
				"pagination":        pagination,
				"sortBy":            "NEWEST",
				"repliesPagination": map[string]interface{}{"first": cfg.RepliesLimit},
			},
			Query: GetCommentsQuery,
		}

		resp, err := PostGraphQL[CommentsResponse](ctx, payload, cfg)
		if err != nil {
			return nil, err
		}

		totalCount = resp.Data.Comments.TotalCount

		for _, edge := range resp.Data.Comments.Edges {
			edges = append(edges, convertCommentEdge(edge))
			if cfg.CommentsLimit > 0 && len(edges) >= cfg.CommentsLimit {
				return &market.CategoryComments{TotalCount: totalCount, Edges: edges}, nil
			}
		}

		if !resp.Data.Comments.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Comments.PageInfo.EndCursor
	}

	// Fetch all replies if configured
	if cfg.FetchAllReplies {
		for i := range edges {
			node := &edges[i].Node
			if node.Replies == nil || !node.Replies.PageInfo.HasNextPage {
				continue
			}

			allReplies := append([]market.CommentReplyEdge{}, node.Replies.Edges...)
			replyCursor := node.Replies.PageInfo.EndCursor

			for replyCursor != nil {
				payload := GQLRequest{
					OperationName: "LoadMoreReplies",
					Variables: map[string]interface{}{
						"commentId": node.ID,
						"after":     *replyCursor,
						"first":     cfg.RepliesLimit,
					},
					Query: LoadMoreRepliesQuery,
				}

				resp, err := PostGraphQL[RepliesResponse](ctx, payload, cfg)
				if err != nil {
					break
				}

				for _, re := range resp.Data.Comment.Replies.Edges {
					allReplies = append(allReplies, market.CommentReplyEdge{
						Node: convertCommentNode(re.Node),
					})
				}

				if !resp.Data.Comment.Replies.PageInfo.HasNextPage {
					replyCursor = nil
				} else {
					replyCursor = resp.Data.Comment.Replies.PageInfo.EndCursor
				}
			}

			node.Replies.Edges = allReplies
			node.Replies.PageInfo.HasNextPage = false
		}
	}

	return &market.CategoryComments{TotalCount: totalCount, Edges: edges}, nil
}

// FetchCategoryTimeseries fetches timeseries for a category and interval.
func FetchCategoryTimeseries(ctx context.Context, cfg config.Config, categoryID string, interval string) (map[string]*market.TimeseriesData, error) {
	result := make(map[string]*market.TimeseriesData)
	var cursor *string

	for {
		pagination := map[string]interface{}{"first": 50}
		if cursor != nil {
			pagination["after"] = *cursor
		}

		payload := GQLRequest{
			OperationName: "GetCategoryTimeseries",
			Variables: map[string]interface{}{
				"categoryId": categoryID,
				"interval":   interval,
				"pagination": pagination,
			},
			Query: GetCategoryTimeseriesQuery,
		}

		resp, err := PostGraphQL[TimeseriesResponse](ctx, payload, cfg)
		if err != nil {
			return nil, err
		}

		for _, edge := range resp.Data.Category.Timeseries.Edges {
			points := make([]market.TimeseriesPoint, len(edge.Node.Data.Edges))
			for i, pe := range edge.Node.Data.Edges {
				x, _ := strconv.ParseFloat(string(pe.Node.X), 64)
				y, _ := strconv.ParseFloat(string(pe.Node.Y), 64)
				points[i] = market.TimeseriesPoint{X: x, Y: y}
			}
			result[edge.Node.Market.ID] = &market.TimeseriesData{
				DataGranularity: edge.Node.DataGranularity,
				Points:          points,
			}
		}

		if !resp.Data.Category.Timeseries.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Category.Timeseries.PageInfo.EndCursor
	}

	return result, nil
}

// jsonNumberToIntPtr converts a json.Number to *int.
func jsonNumberToIntPtr(n *json.Number) *int {
	if n == nil {
		return nil
	}
	s := strings.TrimSpace(n.String())
	if s == "" {
		return nil
	}
	// Try parsing as int directly, or as float then truncate
	if i, err := strconv.Atoi(s); err == nil {
		return &i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		i := int(f)
		return &i
	}
	return nil
}

func jsonNumberToStringPtr(n *json.Number) *string {
	if n == nil {
		return nil
	}
	s := n.String()
	return &s
}

// convertCommentNode converts a GQL comment node to market type.
func convertCommentNode(n CommentNodeGQL) market.CommentNode {
	node := market.CommentNode{
		ID:            n.ID,
		Content:       n.Content,
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
		LikeCount:     n.LikeCount,
		IsLikedByUser: n.IsLikedByUser,
		ReplyCount:    n.ReplyCount,
		ReportCount:   n.ReportCount,
		Account: market.CommentAccount{
			Address:  n.Account.Address,
			Name:     n.Account.Name,
			ImageUrl: n.Account.ImageUrl,
		},
	}

	if n.ParentComment != nil {
		node.ParentComment = &market.CommentRef{ID: n.ParentComment.ID}
	}
	if n.ReplyToComment != nil {
		node.ReplyToComment = &market.ReplyToRef{
			ID: n.ReplyToComment.ID,
			Account: market.ReplyToAccount{
				Name:    n.ReplyToComment.Account.Name,
				Address: n.ReplyToComment.Account.Address,
			},
		}
	}
	if n.Replies != nil {
		replies := &market.CommentReplies{
			PageInfo: market.CommentPageInfo{
				HasNextPage: n.Replies.PageInfo.HasNextPage,
				EndCursor:   n.Replies.PageInfo.EndCursor,
			},
			TotalCount: n.Replies.TotalCount,
			Edges:      []market.CommentReplyEdge{},
		}
		for _, re := range n.Replies.Edges {
			replies.Edges = append(replies.Edges, market.CommentReplyEdge{
				Node: convertCommentNode(re.Node),
			})
		}
		node.Replies = replies
	}

	return node
}

// convertCommentEdge converts a GQL comment edge to market type.
func convertCommentEdge(e CommentEdgeGQL) market.CommentEdge {
	node := convertCommentNode(e.Node)
	// Top-level comments always have a replies object (even if empty)
	if node.Replies == nil {
		node.Replies = &market.CommentReplies{
			Edges: []market.CommentReplyEdge{},
		}
	}
	return market.CommentEdge{
		Cursor: e.Cursor,
		Node:   node,
	}
}

// ToFullView converts a MarketDetail into a NormalizedMarket.
func ToFullView(
	detail MarketDetail,
	orderbook *market.OrderbookView,
	holders *market.MarketHolders,
	comments *market.CategoryComments,
	timeseries map[string]*market.TimeseriesData,
) market.NormalizedMarket {
	outcomes := make([]market.MarketOutcome, len(detail.Outcomes.Edges))
	totalPositions := 0
	for i, edge := range detail.Outcomes.Edges {
		o := edge.Node
		posCount := 0
		if o.Positions != nil {
			posCount = o.Positions.TotalCount
		}
		var sharesCount, posValueUsd *float64
		if o.Statistics != nil {
			sharesCount = &o.Statistics.SharesCount
			posValueUsd = &o.Statistics.PositionsValueUsd
		}
		onChainID := o.OnChainID
		outcomes[i] = market.MarketOutcome{
			ID:                 o.ID,
			Name:               o.Name,
			Index:              o.Index,
			Status:             o.Status,
			OnChainID:          &onChainID,
			BidPriceInCurrency: o.BidPriceInCurrency,
			AskPriceInCurrency: o.AskPriceInCurrency,
			SharesCount:        sharesCount,
			PositionsValueUsd:  posValueUsd,
			PositionsCount:     posCount,
		}
		totalPositions += posCount
	}

	tags := make([]market.Tag, 0)
	if detail.Category.Tags != nil {
		for _, te := range detail.Category.Tags.Edges {
			tags = append(tags, market.Tag{ID: te.Node.ID, Name: te.Node.Name})
		}
	}

	var commentsTotal *int
	if detail.Category.Comments != nil {
		commentsTotal = detail.Category.Comments.TotalCount
	}

	var resolution *market.MarketResolution
	if detail.Resolution != nil {
		resolution = &market.MarketResolution{
			ID:        detail.Resolution.ID,
			Name:      detail.Resolution.Name,
			Index:     detail.Resolution.Index,
			Status:    detail.Resolution.Status,
			CreatedAt: detail.Resolution.CreatedAt,
		}
	}

	statusLogEdges := make([]market.StatusLogEdge, len(detail.StatusLogs.Edges))
	for i, e := range detail.StatusLogs.Edges {
		statusLogEdges[i] = market.StatusLogEdge{
			Node: market.StatusLogNode{
				Status:          e.Node.Status,
				Timestamp:       e.Node.Timestamp,
				TransactionHash: e.Node.TransactionHash,
			},
		}
	}

	bulletins := make([]market.BulletinBoardUpdate, len(detail.BulletinBoardUpdates))
	for i, b := range detail.BulletinBoardUpdates {
		bulletins[i] = market.BulletinBoardUpdate{
			Content:         b.Content,
			CreatedAt:       b.CreatedAt,
			UpdatedAt:       b.UpdatedAt,
			TransactionHash: b.TransactionHash,
		}
	}

	createdAt := detail.CreatedAt

	return market.NormalizedMarket{
		ID:                     detail.ID,
		Title:                  detail.Title,
		Question:               detail.Question,
		Description:            detail.Description,
		ImageUrl:               detail.ImageUrl,
		CreatedAt:              &createdAt,
		Status:                 detail.Status,
		IsTradingEnabled:       detail.IsTradingEnabled,
		ChancePercentage:       &detail.ChancePercentage,
		SpreadThreshold:        detail.SpreadThreshold,
		SpreadThresholdDecimal: market.FormatDecimal(detail.SpreadThreshold, market.WeiDecimals, 6),
		SpreadThresholdPercent: market.FormatPercent(detail.SpreadThreshold, market.WeiDecimals, 2),
		ShareThreshold:         detail.ShareThreshold,
		MakerFeeBps:            detail.MakerFeeBps,
		TakerFeeBps:            detail.TakerFeeBps,
		DecimalPrecision:       detail.DecimalPrecision,
		OracleQuestionID:       detail.OracleQuestionID,
		OracleTxHash:           detail.OracleTxHash,
		ConditionID:            detail.ConditionID,
		ResolverAddress:        detail.ResolverAddress,
		QuestionIndex:          detail.QuestionIndex,
		Category: market.MarketCategory{
			ID:             detail.Category.ID,
			Title:          detail.Category.Title,
			Description:    detail.Category.Description,
			ImageUrl:       detail.Category.ImageUrl,
			IsNegRisk:      detail.Category.IsNegRisk,
			IsYieldBearing: detail.Category.IsYieldBearing,
			StartsAt:       detail.Category.StartsAt,
			EndsAt:         detail.Category.EndsAt,
			Status:         detail.Category.Status,
			HoldersCount:   jsonNumberToStringPtr(detail.Category.HoldersCount),
			CommentsTotal:  commentsTotal,
			Tags:           tags,
		},
		Statistics: market.MarketStatistics{
			TotalLiquidityUsd:         detail.Statistics.TotalLiquidityUsd,
			VolumeTotalUsd:            detail.Statistics.VolumeTotalUsd,
			Volume24hUsd:              detail.Statistics.Volume24hUsd,
			Volume24hChangeUsd:        detail.Statistics.Volume24hChangeUsd,
			PercentageChanceChange24h: detail.Statistics.PercentageChanceChange24h,
		},
		Outcomes:             outcomes,
		TotalPositions:       totalPositions,
		Resolution:           resolution,
		StatusLogs:           &market.StatusLogs{Edges: statusLogEdges},
		BulletinBoardUpdates: bulletins,
		Orderbook:            orderbook,
		Holders:              holders,
		Comments:             comments,
		Timeseries:           timeseries,
	}
}

// ToView converts a NormalizedMarket to a ViewMarket.
func ToView(m market.NormalizedMarket) market.ViewMarket {
	viewOutcomes := make([]market.ViewOutcome, len(m.Outcomes))
	for i, o := range m.Outcomes {
		viewOutcomes[i] = market.ViewOutcome{
			ID:             o.ID,
			Name:           o.Name,
			Index:          o.Index,
			PositionsCount: o.PositionsCount,
		}
	}

	v := market.ViewMarket{
		ID:               m.ID,
		Title:            m.Title,
		Question:         m.Question,
		ImageUrl:         m.ImageUrl,
		Category:         m.Category,
		ChancePercentage: m.ChancePercentage,
		SpreadThreshold:  m.SpreadThreshold,
		SpreadDecimal:    &m.SpreadThresholdDecimal,
		SpreadPercent:    &m.SpreadThresholdPercent,
		MakerFeeBps:      &m.MakerFeeBps,
		TakerFeeBps:      &m.TakerFeeBps,
		IsTradingEnabled: &m.IsTradingEnabled,
		Status:           &m.Status,
		ShareThreshold:   &m.ShareThreshold,
		Statistics:       &m.Statistics,
		Outcomes:         viewOutcomes,
		TotalPositions:   m.TotalPositions,
	}
	if m.Source != "" {
		v.Source = &m.Source
	}
	return v
}
