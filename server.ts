import http, {IncomingMessage, ServerResponse} from 'node:http';
import {readFile, stat} from 'node:fs/promises';
import {createReadStream} from 'node:fs';
import {createHash} from 'node:crypto';
import {spawn, ChildProcess} from 'node:child_process';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const currentDir = path.dirname(fileURLToPath(import.meta.url));
const SITE_DIR = path.join(currentDir, 'site');
const DATA_PATH = path.join(SITE_DIR, 'data', 'markets_pairs.json');

const PORT = Number(process.env.PORT || 8050);
const REFRESH_INTERVAL_MS = Number(
  process.env.REFRESH_INTERVAL_MS || 90_000,
);
const SSE_PING_MS = Number(process.env.SSE_PING_MS || 25_000);
const AUTO_REFRESH = process.env.AUTO_REFRESH !== '0';

const tsxBin = path.join(currentDir, 'node_modules', '.bin', 'tsx');

const fetchArgs: string[] = [
  path.join(currentDir, 'fetch_all_markets.ts'),
  '--concurrency',
  process.env.FETCH_CONCURRENCY || '3',
  '--category-concurrency',
  process.env.CATEGORY_CONCURRENCY || '2',
];

if (process.env.MAX_MARKETS) {
  fetchArgs.push('--max-markets', process.env.MAX_MARKETS);
}
if (process.env.FETCH_SLEEP) {
  fetchArgs.push('--sleep', process.env.FETCH_SLEEP);
}
if (process.env.COMMENTS_LIMIT) {
  fetchArgs.push('--comments-limit', process.env.COMMENTS_LIMIT);
}
if (process.env.HOLDERS_LIMIT) {
  fetchArgs.push('--holders-limit', process.env.HOLDERS_LIMIT);
}
if (process.env.REPLIES_LIMIT) {
  fetchArgs.push('--replies-limit', process.env.REPLIES_LIMIT);
}
if (process.env.SKIP_ORDERBOOK === '1') {
  fetchArgs.push('--skip-orderbook');
}
if (process.env.SKIP_HOLDERS === '1') {
  fetchArgs.push('--skip-holders');
}
if (process.env.SKIP_COMMENTS === '1') {
  fetchArgs.push('--skip-comments');
}
if (process.env.SKIP_TIMESERIES === '1') {
  fetchArgs.push('--skip-timeseries');
}

const MIME_TYPES: Readonly<Record<string, string>> = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.ico': 'image/x-icon',
};

const POLYMARKET_CLOB_URL =
  process.env.POLYMARKET_CLOB_URL || 'https://clob.polymarket.com';
const POLYMARKET_ORDERBOOK_TTL_MS = Number(
  process.env.POLYMARKET_ORDERBOOK_TTL_MS || 15000,
);
const POLYMARKET_ORDERBOOK_CONCURRENCY = Number(
  process.env.POLYMARKET_ORDERBOOK_CONCURRENCY || 4,
);
const POLYMARKET_ORDERBOOK_LEVELS = Number(
  process.env.POLYMARKET_ORDERBOOK_LEVELS || 8,
);
const POLYMARKET_ORDERBOOK_MAX_TOKENS = Number(
  process.env.POLYMARKET_ORDERBOOK_MAX_TOKENS || 6,
);

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface CacheData {
  generatedAt?: string;
  count?: number;
  pairs?: unknown[];
  markets?: unknown[];
}

interface ServerCache {
  json: string | null;
  data: CacheData | null;
  etag: string | null;
  generatedAt: string | null;
  count: number;
  loadedAt: string | null;
}

interface SseClient {
  res: ServerResponse;
  ping: ReturnType<typeof setInterval> | null;
}

interface NormalizedOrderbookRow {
  price: number;
  size: number;
}

interface NormalizedOrderbook {
  updateTimestampMs: number | null;
  orderCount: number | null;
  lastOrderSettled: null;
  bestAsk: number | null;
  bestBid: number | null;
  spread: number | null;
  spreadCents: number | null;
  asks: NormalizedOrderbookRow[];
  bids: NormalizedOrderbookRow[];
  settlementsPending: null;
}

interface PolymarketBookPayload {
  timestamp?: number;
  error?: string;
  asks?: {price: string; size: string}[];
  bids?: {price: string; size: string}[];
}

interface OrderbookCacheEntry {
  data: NormalizedOrderbook;
  expiresAt: number;
}

interface OrderbookResult {
  tokenId: string;
  ok: boolean;
  orderbook?: NormalizedOrderbook;
  error?: string;
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

const cache: ServerCache = {
  json: null,
  data: null,
  etag: null,
  generatedAt: null,
  count: 0,
  loadedAt: null,
};

let updating = false;
let lastError: Error | null = null;

const orderbookCache = new Map<string, OrderbookCacheEntry>();

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const hashJson = (payload: string): string =>
  createHash('sha1').update(payload).digest('hex');

const loadCacheFromDisk = async (): Promise<void> => {
  const json = await readFile(DATA_PATH, 'utf-8');
  cache.json = json;
  cache.etag = hashJson(json);
  cache.data = JSON.parse(json) as CacheData;
  cache.generatedAt = cache.data?.generatedAt ?? null;
  cache.count =
    cache.data?.count ??
    cache.data?.pairs?.length ??
    cache.data?.markets?.length ??
    0;
  cache.loadedAt = new Date().toISOString();
};

const runFetchScript = (): Promise<void> =>
  new Promise<void>((resolve, reject) => {
    const child: ChildProcess = spawn(tsxBin, fetchArgs, {
      cwd: currentDir,
      stdio: ['ignore', 'pipe', 'pipe'],
    });

    child.stdout?.on('data', (chunk: Buffer) =>
      process.stdout.write(chunk),
    );
    child.stderr?.on('data', (chunk: Buffer) =>
      process.stderr.write(chunk),
    );

    child.on('error', reject);
    child.on('close', (code: number | null) => {
      if (code === 0) {
        resolve();
      } else {
        reject(
          new Error(`fetch_all_markets.ts exited with ${code}`),
        );
      }
    });
  });

// ---------------------------------------------------------------------------
// SSE
// ---------------------------------------------------------------------------

const clients = new Set<SseClient>();

const broadcast = (event: string, payload: unknown): void => {
  const data = `event: ${event}\ndata: ${JSON.stringify(payload)}\n\n`;
  for (const client of clients) {
    client.res.write(data);
  }
};

// ---------------------------------------------------------------------------
// Concurrency helper
// ---------------------------------------------------------------------------

const withConcurrency = async <T, R>(
  items: T[],
  limit: number,
  task: (item: T, index: number) => Promise<R>,
): Promise<R[]> => {
  const results: R[] = new Array(items.length) as R[];
  let index = 0;

  const workers = Array.from(
    {length: Math.max(1, limit)},
    async () => {
      while (index < items.length) {
        const current = index;
        index += 1;
        results[current] = await task(items[current], current);
      }
    },
  );

  await Promise.all(workers);
  return results;
};

// ---------------------------------------------------------------------------
// Polymarket orderbook
// ---------------------------------------------------------------------------

const normalizePolymarketBook = (
  payload: PolymarketBookPayload,
): NormalizedOrderbook => {
  const mapRow = (
    row: {price: string; size: string},
  ): NormalizedOrderbookRow => ({
    price: Number(row.price),
    size: Number(row.size),
  });

  const asks = (payload.asks ?? [])
    .map(mapRow)
    .filter(
      (row) => Number.isFinite(row.price) && Number.isFinite(row.size),
    )
    .slice(0, POLYMARKET_ORDERBOOK_LEVELS);
  const bids = (payload.bids ?? [])
    .map(mapRow)
    .filter(
      (row) => Number.isFinite(row.price) && Number.isFinite(row.size),
    )
    .slice(0, POLYMARKET_ORDERBOOK_LEVELS);

  const bestAsk = asks.length ? asks[0].price : null;
  const bestBid = bids.length ? bids[0].price : null;
  const spread =
    bestAsk !== null && bestBid !== null
      ? Number((bestAsk - bestBid).toFixed(6))
      : null;
  const spreadCents =
    spread !== null ? Number((spread * 100).toFixed(4)) : null;

  return {
    updateTimestampMs: payload.timestamp ?? null,
    orderCount: null,
    lastOrderSettled: null,
    bestAsk,
    bestBid,
    spread,
    spreadCents,
    asks,
    bids,
    settlementsPending: null,
  };
};

const fetchPolymarketOrderbook = async (
  tokenId: string,
): Promise<NormalizedOrderbook> => {
  const cached = orderbookCache.get(tokenId);
  if (cached && cached.expiresAt > Date.now()) {
    return cached.data;
  }

  const res = await fetch(
    `${POLYMARKET_CLOB_URL}/book?token_id=${encodeURIComponent(tokenId)}`,
    {headers: {'User-Agent': 'predict-market'}},
  );
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`Polymarket ${res.status}: ${text}`);
  }
  const payload = (await res.json()) as PolymarketBookPayload;
  if (payload.error) {
    throw new Error(payload.error);
  }

  const data = normalizePolymarketBook(payload);
  orderbookCache.set(tokenId, {
    data,
    expiresAt: Date.now() + POLYMARKET_ORDERBOOK_TTL_MS,
  });
  return data;
};

const handlePolymarketOrderbook = async (
  url: URL,
  res: ServerResponse,
): Promise<void> => {
  const tokenParam =
    url.searchParams.get('token_ids') ||
    url.searchParams.get('token_id');

  if (!tokenParam) {
    res.writeHead(400, {'Content-Type': 'application/json'});
    res.end(JSON.stringify({error: 'token_ids is required'}));
    return;
  }

  const tokenIds = tokenParam
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0)
    .slice(0, POLYMARKET_ORDERBOOK_MAX_TOKENS);

  const results: OrderbookResult[] = await withConcurrency(
    tokenIds,
    POLYMARKET_ORDERBOOK_CONCURRENCY,
    async (tokenId: string): Promise<OrderbookResult> => {
      try {
        const orderbook = await fetchPolymarketOrderbook(tokenId);
        return {tokenId, ok: true, orderbook};
      } catch (error) {
        const message =
          error instanceof Error ? error.message : String(error);
        return {tokenId, ok: false, error: message};
      }
    },
  );

  res.writeHead(200, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
  });
  res.end(JSON.stringify({data: results}));
};

// ---------------------------------------------------------------------------
// Data update
// ---------------------------------------------------------------------------

const updateData = async (): Promise<boolean> => {
  if (updating) return false;
  updating = true;
  broadcast('status', {state: 'updating', at: new Date().toISOString()});

  try {
    await runFetchScript();
    await loadCacheFromDisk();
    lastError = null;
    broadcast('update', {
      generatedAt: cache.generatedAt,
      count: cache.count,
      loadedAt: cache.loadedAt,
    });
    return true;
  } catch (error) {
    lastError = error instanceof Error ? error : new Error(String(error));
    broadcast('status', {
      state: 'error',
      message: lastError.message,
      at: new Date().toISOString(),
    });
    return false;
  } finally {
    updating = false;
  }
};

const shouldRefreshOnStart = async (): Promise<boolean> => {
  try {
    const info = await stat(DATA_PATH);
    const ageMs = Date.now() - info.mtimeMs;
    return ageMs > REFRESH_INTERVAL_MS * 0.6;
  } catch {
    return true;
  }
};

// ---------------------------------------------------------------------------
// Request handlers
// ---------------------------------------------------------------------------

const serveMarkets = async (
  req: IncomingMessage,
  res: ServerResponse,
): Promise<void> => {
  if (!cache.json) {
    try {
      await loadCacheFromDisk();
    } catch {
      res.writeHead(503, {'Content-Type': 'application/json'});
      res.end(
        JSON.stringify({error: 'Market data not ready yet.'}),
      );
      return;
    }
  }

  if (req.headers['if-none-match'] === cache.etag) {
    res.writeHead(304);
    res.end();
    return;
  }

  res.writeHead(200, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
    ETag: cache.etag!,
  });
  res.end(cache.json);
};

const serveStatus = (res: ServerResponse): void => {
  res.writeHead(200, {'Content-Type': 'application/json'});
  res.end(
    JSON.stringify({
      generatedAt: cache.generatedAt,
      count: cache.count,
      updating,
      loadedAt: cache.loadedAt,
      lastError: lastError ? lastError.message : null,
    }),
  );
};

const handleStream = (
  req: IncomingMessage,
  res: ServerResponse,
): void => {
  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-store',
    Connection: 'keep-alive',
  });

  const client: SseClient = {res, ping: null};
  clients.add(client);

  res.write(
    `event: hello\ndata: ${JSON.stringify({
      generatedAt: cache.generatedAt,
      count: cache.count,
      loadedAt: cache.loadedAt,
    })}\n\n`,
  );

  client.ping = setInterval(() => {
    res.write(': ping\n\n');
  }, SSE_PING_MS);

  req.on('close', () => {
    if (client.ping) {
      clearInterval(client.ping);
    }
    clients.delete(client);
  });
};

const serveStatic = async (
  pathname: string,
  res: ServerResponse,
): Promise<void> => {
  const cleanPath = pathname === '/' ? '/index.html' : pathname;
  const safePath = path
    .normalize(cleanPath)
    .replace(/^(\.\.[/\\])+/, '');
  const filePath = path.join(SITE_DIR, safePath);

  if (!filePath.startsWith(SITE_DIR)) {
    res.writeHead(403);
    res.end('Forbidden');
    return;
  }

  try {
    const fileStat = await stat(filePath);
    if (fileStat.isDirectory()) {
      res.writeHead(403);
      res.end('Forbidden');
      return;
    }

    const ext = path.extname(filePath);
    const contentType =
      MIME_TYPES[ext] || 'application/octet-stream';
    const cacheControl =
      ext === '.json' ? 'no-store' : 'public, max-age=300';

    res.writeHead(200, {
      'Content-Type': contentType,
      'Cache-Control': cacheControl,
    });
    createReadStream(filePath).pipe(res);
  } catch {
    res.writeHead(404);
    res.end('Not found');
  }
};

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

const server = http.createServer(
  async (req: IncomingMessage, res: ServerResponse) => {
    const url = new URL(
      req.url ?? '/',
      `http://${req.headers.host}`,
    );
    const pathname = decodeURIComponent(url.pathname);

    if (pathname === '/api/markets' && req.method === 'GET') {
      await serveMarkets(req, res);
      return;
    }

    if (pathname === '/api/status' && req.method === 'GET') {
      serveStatus(res);
      return;
    }

    if (pathname === '/api/refresh' && req.method === 'POST') {
      if (updating) {
        res.writeHead(409, {'Content-Type': 'application/json'});
        res.end(JSON.stringify({status: 'busy'}));
        return;
      }
      updateData();
      res.writeHead(202, {'Content-Type': 'application/json'});
      res.end(JSON.stringify({status: 'started'}));
      return;
    }

    if (pathname === '/api/stream' && req.method === 'GET') {
      handleStream(req, res);
      return;
    }

    if (
      pathname === '/api/polymarket/orderbook' &&
      req.method === 'GET'
    ) {
      await handlePolymarketOrderbook(url, res);
      return;
    }

    await serveStatic(pathname, res);
  },
);

server.listen(PORT, async () => {
  try {
    await loadCacheFromDisk();
  } catch {
    // Data will be generated on first refresh.
  }

  if (AUTO_REFRESH) {
    if (await shouldRefreshOnStart()) {
      updateData();
    }
    setInterval(updateData, REFRESH_INTERVAL_MS);
  }

  console.log(
    `Server listening on http://localhost:${PORT} (refresh every ${REFRESH_INTERVAL_MS}ms)`,
  );
});
