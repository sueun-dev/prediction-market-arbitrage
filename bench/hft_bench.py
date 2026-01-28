#!/usr/bin/env python3
import json, time, resource, urllib.request

GRAPHQL_URL = "https://graphql.predict.fun/graphql"
QUERY = """query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo { hasNextPage endCursor }
    edges { node { id status isTradingEnabled category { id } } }
  }
}""".strip()

def main():
    t0 = time.perf_counter()
    latencies = []
    markets = []
    cursor = None
    first_data_ms = 0

    while True:
        pagination = {"first": 50}
        if cursor:
            pagination["after"] = cursor

        body = json.dumps({
            "operationName": "GetMarkets",
            "variables": {"filter": {"isResolved": False}, "sort": "VOLUME_24H_DESC", "pagination": pagination},
            "query": QUERY,
        }).encode("utf-8")

        req_start = time.perf_counter()
        req = urllib.request.Request(GRAPHQL_URL, data=body, headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read().decode("utf-8"))
        req_end = time.perf_counter()
        latencies.append((req_end - req_start) * 1000)

        page = data["data"]["markets"]
        if not first_data_ms and page["edges"]:
            first_data_ms = (req_end - t0) * 1000

        for edge in page["edges"]:
            markets.append(edge["node"])

        if not page["pageInfo"]["hasNextPage"]:
            break
        cursor = page["pageInfo"]["endCursor"]

    elapsed = time.perf_counter() - t0
    latencies.sort()
    n = len(latencies)
    avg_lat = sum(latencies) / n
    rss_kb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss // 1024  # macOS reports bytes

    print(json.dumps({
        "lang": "Python",
        "markets": len(markets),
        "totalSec": round(elapsed, 3),
        "requests": n,
        "firstDataMs": round(first_data_ms, 1),
        "avgLatMs": round(avg_lat, 1),
        "minLatMs": round(latencies[0], 1),
        "maxLatMs": round(latencies[-1], 1),
        "p50Ms": round(latencies[n // 2], 1),
        "p99Ms": round(latencies[int(n * 0.99)], 1),
        "rssKb": rss_kb,
        "heapKb": 0,
    }))

if __name__ == "__main__":
    main()
