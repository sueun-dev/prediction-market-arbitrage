#!/usr/bin/env python3
import json
import time
import urllib.request

GRAPHQL_URL = "https://graphql.predict.fun/graphql"

QUERY = """
query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo { hasNextPage endCursor }
    edges { node { id status isTradingEnabled category { id } } }
  }
}""".strip()


def main():
    start = time.perf_counter()
    markets = []
    cursor = None

    while True:
        pagination = {"first": 50}
        if cursor:
            pagination["after"] = cursor

        payload = json.dumps({
            "operationName": "GetMarkets",
            "variables": {"filter": {"isResolved": False}, "sort": "VOLUME_24H_DESC", "pagination": pagination},
            "query": QUERY,
        }).encode("utf-8")

        req = urllib.request.Request(GRAPHQL_URL, data=payload, headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read().decode("utf-8"))

        page = data["data"]["markets"]
        for edge in page["edges"]:
            markets.append(edge["node"])

        if not page["pageInfo"]["hasNextPage"]:
            break
        cursor = page["pageInfo"]["endCursor"]

    elapsed = time.perf_counter() - start
    print(f"Python: {len(markets)} markets in {elapsed:.3f}s")


if __name__ == "__main__":
    main()
