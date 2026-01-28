#!/usr/bin/env python3
import argparse
import json
import sys
import time
import urllib.request
from typing import Any, Dict, List, Optional

GRAPHQL_URL = "https://graphql.predict.fun/graphql"

GET_MARKETS_QUERY = """
query GetMarkets(
  $filter: MarketFilterInput
  $sort: MarketSortInput
  $pagination: ForwardPaginationInput
) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo {
      hasNextPage
      startCursor
      endCursor
    }
    edges {
      node {
        ...Market
      }
    }
  }
}

fragment Market on Market {
  id
  decimalPrecision
  category {
    id
    imageUrl
    isNegRisk
    isYieldBearing
    title
  }
  title
  question
  imageUrl
  chancePercentage
  spreadThreshold
  makerFeeBps
  takerFeeBps
  isTradingEnabled
  status
  shareThreshold
  statistics {
    percentageChanceChange24h
    volume24hUsd
    volume24hChangeUsd
    volumeTotalUsd
    totalLiquidityUsd
  }
  outcomes {
    edges {
      node {
        ...MarketOutcome
      }
    }
  }
  resolution {
    id
    name
    index
    status
    createdAt
  }
}

fragment MarketOutcome on Outcome {
  id
  name
  index
  onChainId
  status
  positions {
    totalCount
  }
}
""".strip()


class GraphQLError(RuntimeError):
    pass


def post_graphql(payload: Dict[str, Any], timeout: int, retries: int, backoff: float) -> Dict[str, Any]:
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        GRAPHQL_URL, data=body, headers={"Content-Type": "application/json"}
    )

    attempt = 0
    while True:
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                data = json.loads(resp.read().decode("utf-8"))
            if "errors" in data:
                raise GraphQLError(json.dumps(data["errors"], ensure_ascii=True))
            return data
        except Exception as exc:
            attempt += 1
            if attempt > retries:
                raise
            time.sleep(backoff * attempt)


def fetch_all_markets(
    page_size: int,
    sort: str,
    sleep_seconds: float,
    timeout: int,
    retries: int,
    backoff: float,
) -> List[Dict[str, Any]]:
    markets: List[Dict[str, Any]] = []
    cursor: Optional[str] = None

    while True:
        pagination: Dict[str, Any] = {"first": page_size}
        if cursor:
            pagination["after"] = cursor

        variables = {
            "filter": {"isResolved": False},
            "sort": sort,
            "pagination": pagination,
        }
        payload = {
            "operationName": "GetMarkets",
            "variables": variables,
            "query": GET_MARKETS_QUERY,
        }
        data = post_graphql(payload, timeout=timeout, retries=retries, backoff=backoff)
        page = data["data"]["markets"]
        edges = page.get("edges", [])
        for edge in edges:
            markets.append(edge["node"])

        page_info = page.get("pageInfo", {})
        if not page_info.get("hasNextPage"):
            break

        cursor = page_info.get("endCursor")
        if sleep_seconds > 0:
            time.sleep(sleep_seconds)

    return markets


def filter_bettable(markets: List[Dict[str, Any]], status: str) -> List[Dict[str, Any]]:
    out: List[Dict[str, Any]] = []
    for m in markets:
        if not m.get("isTradingEnabled"):
            continue
        if status and m.get("status") != status:
            continue
        out.append(m)
    return out


def write_output(markets: List[Dict[str, Any]], out_path: Optional[str], fmt: str) -> None:
    if fmt == "ndjson":
        lines = "\n".join(json.dumps(m, ensure_ascii=True) for m in markets) + "\n"
        if out_path:
            with open(out_path, "w", encoding="utf-8") as f:
                f.write(lines)
        else:
            sys.stdout.write(lines)
        return

    payload = {"count": len(markets), "markets": markets}
    text = json.dumps(payload, ensure_ascii=True)
    if out_path:
        with open(out_path, "w", encoding="utf-8") as f:
            f.write(text)
    else:
        sys.stdout.write(text)


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="Fetch all bettable markets from predict.fun GraphQL",
    )
    p.add_argument("--page-size", type=int, default=50)
    p.add_argument("--sort", default="VOLUME_24H_DESC")
    p.add_argument("--sleep", type=float, default=0.0)
    p.add_argument("--timeout", type=int, default=20)
    p.add_argument("--retries", type=int, default=3)
    p.add_argument("--backoff", type=float, default=0.5)
    p.add_argument(
        "--status",
        default="",
        help="Optional Market.status filter (leave empty to include all statuses)",
    )
    p.add_argument("--include-nonbettable", action="store_true")
    p.add_argument("--format", choices=["json", "ndjson"], default="json")
    p.add_argument("--out", help="Write output to file path instead of stdout")
    return p.parse_args()


def main() -> int:
    args = parse_args()
    markets = fetch_all_markets(
        page_size=args.page_size,
        sort=args.sort,
        sleep_seconds=args.sleep,
        timeout=args.timeout,
        retries=args.retries,
        backoff=args.backoff,
    )

    if not args.include_nonbettable:
        markets = filter_bettable(markets, status=args.status)

    write_output(markets, out_path=args.out, fmt=args.format)

    status_label = args.status if args.status else "ANY"
    print(
        f"fetched={len(markets)} sort={args.sort} status={status_label} format={args.format}",
        file=sys.stderr,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
