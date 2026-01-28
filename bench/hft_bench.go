package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
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
	ID               string `json:"id"`
	Status           string `json:"status"`
	IsTradingEnabled bool   `json:"isTradingEnabled"`
	Category         struct{ ID string `json:"id"` } `json:"category"`
}
type marketsResponse struct {
	Data struct {
		Markets struct {
			PageInfo pageInfo                    `json:"pageInfo"`
			Edges    []struct{ Node marketNode `json:"node"` } `json:"edges"`
		} `json:"markets"`
	} `json:"data"`
}
type gqlRequest struct {
	OperationName string      `json:"operationName"`
	Variables     interface{} `json:"variables"`
	Query         string      `json:"query"`
}

type result struct {
	Lang        string  `json:"lang"`
	Markets     int     `json:"markets"`
	TotalSec    float64 `json:"totalSec"`
	Requests    int     `json:"requests"`
	FirstDataMs float64 `json:"firstDataMs"`
	AvgLatMs    float64 `json:"avgLatMs"`
	MinLatMs    float64 `json:"minLatMs"`
	MaxLatMs    float64 `json:"maxLatMs"`
	P50Ms       float64 `json:"p50Ms"`
	P99Ms       float64 `json:"p99Ms"`
	RssKb       int64   `json:"rssKb"`
	HeapKb      int64   `json:"heapKb"`
}

func main() {
	t0 := time.Now()
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	var markets []marketNode
	var cursor *string
	var latencies []float64
	var firstDataMs float64

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
		body, _ := json.Marshal(gqlRequest{OperationName: "GetMarkets", Variables: variables, Query: query})

		reqStart := time.Now()
		resp, err := client.Post(graphqlURL, "application/json", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		reqEnd := time.Now()

		latMs := float64(reqEnd.Sub(reqStart).Microseconds()) / 1000.0
		latencies = append(latencies, latMs)

		var res marketsResponse
		json.Unmarshal(data, &res)

		if firstDataMs == 0 && len(res.Data.Markets.Edges) > 0 {
			firstDataMs = float64(reqEnd.Sub(t0).Microseconds()) / 1000.0
		}
		for _, edge := range res.Data.Markets.Edges {
			markets = append(markets, edge.Node)
		}
		if !res.Data.Markets.PageInfo.HasNextPage {
			break
		}
		cursor = res.Data.Markets.PageInfo.EndCursor
	}

	elapsed := time.Since(t0).Seconds()
	sort.Float64s(latencies)
	n := len(latencies)
	sum := 0.0
	for _, v := range latencies {
		sum += v
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	out, _ := json.Marshal(result{
		Lang:        "Go",
		Markets:     len(markets),
		TotalSec:    float64(int(elapsed*1000)) / 1000,
		Requests:    n,
		FirstDataMs: float64(int(firstDataMs*10)) / 10,
		AvgLatMs:    float64(int(sum/float64(n)*10)) / 10,
		MinLatMs:    float64(int(latencies[0]*10)) / 10,
		MaxLatMs:    float64(int(latencies[n-1]*10)) / 10,
		P50Ms:       float64(int(latencies[n/2]*10)) / 10,
		P99Ms:       float64(int(latencies[int(float64(n)*0.99)]*10)) / 10,
		RssKb:       int64(mem.Sys / 1024),
		HeapKb:      int64(mem.HeapAlloc / 1024),
	})
	fmt.Println(string(out))
}
