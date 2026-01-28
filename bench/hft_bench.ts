const GRAPHQL_URL = 'https://graphql.predict.fun/graphql';

const QUERY = `
query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo { hasNextPage endCursor }
    edges { node { id status isTradingEnabled category { id } } }
  }
}`.trim();

interface PageInfo { hasNextPage: boolean; endCursor: string | null; }
interface MarketNode { id: string; status: string; isTradingEnabled: boolean; category: {id: string}; }
interface MarketsResponse { data: { markets: { pageInfo: PageInfo; edges: {node: MarketNode}[]; }; }; }

const main = async (): Promise<void> => {
  const t0 = performance.now();
  const latencies: number[] = [];
  const markets: MarketNode[] = [];
  let cursor: string | null = null;
  let firstDataAt = 0;

  while (true) {
    const pagination: Record<string, unknown> = {first: 50};
    if (cursor) pagination.after = cursor;

    const reqStart = performance.now();
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
    const reqEnd = performance.now();
    latencies.push(reqEnd - reqStart);

    if (!firstDataAt && json.data.markets.edges.length > 0) {
      firstDataAt = reqEnd - t0;
    }

    for (const edge of json.data.markets.edges) {
      markets.push(edge.node);
    }

    if (!json.data.markets.pageInfo.hasNextPage) break;
    cursor = json.data.markets.pageInfo.endCursor;
  }

  const elapsed = (performance.now() - t0) / 1000;
  const avgLat = latencies.reduce((a, b) => a + b, 0) / latencies.length;
  const minLat = Math.min(...latencies);
  const maxLat = Math.max(...latencies);
  const p50 = latencies.sort((a, b) => a - b)[Math.floor(latencies.length * 0.5)];
  const p99 = latencies.sort((a, b) => a - b)[Math.floor(latencies.length * 0.99)];
  const mem = process.memoryUsage();

  console.log(JSON.stringify({
    lang: 'TypeScript',
    markets: markets.length,
    totalSec: Number(elapsed.toFixed(3)),
    requests: latencies.length,
    firstDataMs: Number(firstDataAt.toFixed(1)),
    avgLatMs: Number(avgLat.toFixed(1)),
    minLatMs: Number(minLat.toFixed(1)),
    maxLatMs: Number(maxLat.toFixed(1)),
    p50Ms: Number(p50.toFixed(1)),
    p99Ms: Number(p99.toFixed(1)),
    rssKb: Math.round(mem.rss / 1024),
    heapKb: Math.round(mem.heapUsed / 1024),
  }));
};

await main();
