const GRAPHQL_URL = 'https://graphql.predict.fun/graphql';

const QUERY = `
query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo { hasNextPage endCursor }
    edges { node { id status isTradingEnabled category { id } } }
  }
}`.trim();

interface PageInfo {
  hasNextPage: boolean;
  endCursor: string | null;
}

interface MarketNode {
  id: string;
  status: string;
  isTradingEnabled: boolean;
  category: {id: string};
}

interface MarketsResponse {
  data: {
    markets: {
      pageInfo: PageInfo;
      edges: {node: MarketNode}[];
    };
  };
}

const main = async (): Promise<void> => {
  const start = performance.now();
  const markets: MarketNode[] = [];
  let cursor: string | null = null;

  while (true) {
    const pagination: Record<string, unknown> = {first: 50};
    if (cursor) pagination.after = cursor;

    const res = await fetch(GRAPHQL_URL, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({
        operationName: 'GetMarkets',
        variables: {filter: {isResolved: false}, sort: 'VOLUME_24H_DESC', pagination},
        query: QUERY,
      }),
    });

    const json = (await res.json()) as MarketsResponse;
    const page = json.data.markets;
    for (const edge of page.edges) {
      markets.push(edge.node);
    }

    if (!page.pageInfo.hasNextPage) break;
    cursor = page.pageInfo.endCursor;
  }

  const elapsed = ((performance.now() - start) / 1000).toFixed(3);
  console.log(`TypeScript: ${markets.length} markets in ${elapsed}s`);
};

await main();
