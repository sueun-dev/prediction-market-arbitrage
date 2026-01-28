package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const graphqlURL = "https://graphql.predict.fun/graphql"

const query = `query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo { hasNextPage endCursor }
    edges { node { id status isTradingEnabled category { id } } }
  }
}`

type pageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

type marketNode struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	IsTradingEnabled  bool   `json:"isTradingEnabled"`
	Category          struct {
		ID string `json:"id"`
	} `json:"category"`
}

type marketsResponse struct {
	Data struct {
		Markets struct {
			PageInfo pageInfo         `json:"pageInfo"`
			Edges    []struct {
				Node marketNode `json:"node"`
			} `json:"edges"`
		} `json:"markets"`
	} `json:"data"`
}

type gqlRequest struct {
	OperationName string      `json:"operationName"`
	Variables     interface{} `json:"variables"`
	Query         string      `json:"query"`
}

func main() {
	t0 := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}

	var markets []marketNode
	var cursor *string

	for {
		pagination := map[string]interface{}{"first": 50}
		if cursor != nil {
			pagination["after"] = *cursor
		}

		variables := map[string]interface{}{
			"filter":     map[string]interface{}{"isResolved": false},
			"sort":       "VOLUME_24H_DESC",
			"pagination": pagination,
		}

		body, _ := json.Marshal(gqlRequest{
			OperationName: "GetMarkets",
			Variables:     variables,
			Query:         query,
		})

		resp, err := client.Post(graphqlURL, "application/json", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}

		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result marketsResponse
		if err := json.Unmarshal(data, &result); err != nil {
			fmt.Printf("json error: %v\n", err)
			return
		}

		for _, edge := range result.Data.Markets.Edges {
			markets = append(markets, edge.Node)
		}

		if !result.Data.Markets.PageInfo.HasNextPage {
			break
		}
		cursor = result.Data.Markets.PageInfo.EndCursor
	}

	elapsed := time.Since(t0).Seconds()
	_ = strings.TrimSpace("")
	fmt.Printf("Go: %d markets in %.3fs\n", len(markets), elapsed)
}
