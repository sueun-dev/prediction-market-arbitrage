import {mkdir, readFile, writeFile} from 'node:fs/promises';
import {spawn, ChildProcess} from 'node:child_process';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const currentDir = path.dirname(fileURLToPath(import.meta.url));
const DATA_DIR = path.join(currentDir, 'site', 'data');
const PREDICT_FULL = path.join(DATA_DIR, 'predict_markets_full.json');
const PREDICT_VIEW = path.join(DATA_DIR, 'predict_markets_view.json');
const OUT_FULL = path.join(DATA_DIR, 'markets_full.json');
const OUT_VIEW = path.join(DATA_DIR, 'markets_view.json');
const PAIRS_OUT = path.join(DATA_DIR, 'markets_pairs.json');

const POLYMARKET_API =
  process.env.POLYMARKET_API || 'https://gamma-api.polymarket.com';
const POLYMARKET_PAGE_SIZE = Number(process.env.POLYMARKET_PAGE_SIZE || 100);
const POLYMARKET_MAX_MARKETS = Number(
  process.env.POLYMARKET_MAX_MARKETS || 0,
);
const POLYMARKET_ACTIVE_ONLY = process.env.POLYMARKET_ACTIVE_ONLY !== '0';
const POLYMARKET_ACCEPTING_ONLY =
  process.env.POLYMARKET_ACCEPTING_ONLY !== '0';
const PAIR_MIN_SIMILARITY = Number(process.env.PAIR_MIN_SIMILARITY || 0.8);
const PAIR_MIN_CHAR_SIMILARITY = Number(
  process.env.PAIR_MIN_CHAR_SIMILARITY || 0.78,
);
const PAIR_MIN_MARGIN = Number(process.env.PAIR_MIN_MARGIN || 0.08);
const PAIR_MIN_TOKENS = Number(process.env.PAIR_MIN_TOKENS || 4);
const PAIR_REQUIRE_NUMBER_MATCH =
  process.env.PAIR_REQUIRE_NUMBER_MATCH !== '0';
const PAIR_REQUIRE_YEAR_MATCH = process.env.PAIR_REQUIRE_YEAR_MATCH !== '0';

// ---------------------------------------------------------------------------
// Shared types imported from fetch_open_markets.ts conceptually.
// We re-declare the shapes we actually use from the predict data.
// ---------------------------------------------------------------------------

interface MarketStatistics {
  totalLiquidityUsd: number;
  volumeTotalUsd: number;
  volume24hUsd: number;
  volume24hChangeUsd: number;
  percentageChanceChange24h: number | null;
}

interface MarketCategory {
  id: string;
  title: string;
  description: string | null;
  imageUrl: string;
  isNegRisk: boolean;
  isYieldBearing: boolean;
  startsAt: string | null;
  endsAt: string | null;
  status: string;
  holdersCount: number | null;
  commentsTotal: number | null;
  tags: {id: string; name: string}[];
}

interface OrderbookView {
  updateTimestampMs: number | null;
  orderCount: number | null;
  lastOrderSettled: {price: string; side: string} | null;
  bestAsk: number | null;
  bestBid: number | null;
  spread: number | null;
  spreadCents: number | null;
  asks: {price: number; size: number}[];
  bids: {price: number; size: number}[];
  settlementsPending: number | null;
}

interface MarketOutcome {
  id: string;
  name: string;
  index: number;
  status: string | null;
  onChainId: string | null;
  bidPriceInCurrency: number | null;
  askPriceInCurrency: number | null;
  sharesCount: number | null;
  positionsValueUsd: number | null;
  positionsCount: number;
  price?: number | null;
  bidPrice?: number | null;
  askPrice?: number | null;
}

interface HolderPosition {
  id: string;
  shares: string;
  valueUsd: number | null;
  sharesDecimal: string;
  account: {address: string; name: string | null};
}

interface MarketHolders {
  outcomes: {
    outcomeId: string;
    index: number;
    name: string;
    totalCount: number;
    positions: HolderPosition[];
  }[];
}

interface CategoryComments {
  totalCount: number;
  edges: {
    cursor: string;
    node: {
      id: string;
      content: string;
      account: {address: string; name: string | null};
    };
  }[];
}

interface TimeseriesInterval {
  dataGranularity: string;
  points: {x: number; y: number}[];
}

interface MarketResolution {
  id: string;
  name: string;
  index: number;
  status: string;
  createdAt: string;
}

interface StatusLogEdge {
  node: {
    status: string;
    timestamp: string;
    transactionHash: string | null;
    outcome: {
      id: string;
      index: number;
      name: string;
      status: string | null;
    } | null;
  };
}

interface BulletinBoardUpdate {
  content: string;
  createdAt: string;
  updatedAt: string | null;
  transactionHash: string | null;
}

interface NormalizedMarket {
  id: string;
  title: string;
  question: string;
  description: string | null;
  imageUrl: string;
  createdAt: string | null;
  status: string;
  isTradingEnabled: boolean;
  chancePercentage: number | null;
  spreadThreshold: string;
  spreadThresholdDecimal: string;
  spreadThresholdPercent: string;
  shareThreshold: string;
  makerFeeBps: number;
  takerFeeBps: number;
  decimalPrecision: number;
  oracleQuestionId: string | null;
  oracleTxHash: string | null;
  conditionId: string | null;
  resolverAddress: string | null;
  questionIndex: number | null;
  category: MarketCategory;
  statistics: MarketStatistics;
  outcomes: MarketOutcome[];
  totalPositions: number;
  resolution: MarketResolution | null;
  statusLogs: {edges: StatusLogEdge[]} | null;
  bulletinBoardUpdates: BulletinBoardUpdate[];
  orderbook: OrderbookView | null;
  holders: MarketHolders | null;
  comments: CategoryComments | null;
  timeseries: Partial<Record<string, TimeseriesInterval>> | null;
  source: string;
  sourceUrl: string;
  orderbookTokens?: {tokenId: string; outcome: string}[];
}

// Predict-specific output payload from fetch_open_markets
interface PredictFullPayload {
  generatedAt: string;
  count: number;
  markets: NormalizedMarket[];
}

// ---------------------------------------------------------------------------
// Polymarket raw API types
// ---------------------------------------------------------------------------

interface PolymarketRawMarket {
  id: string;
  question?: string;
  description?: string;
  image?: string;
  icon?: string;
  createdAt?: string;
  closed?: boolean;
  acceptingOrders?: boolean;
  active?: boolean;
  outcomes?: string | string[];
  outcomePrices?: string | (string | number)[];
  clobTokenIds?: string | string[];
  bestAsk?: string | number;
  bestBid?: string | number;
  liquidityNum?: number;
  liquidity?: string | number;
  volumeNum?: number;
  volume?: string | number;
  volume24hr?: number;
  volume24hrClob?: number;
  lastTradePrice?: number;
  spread?: number;
  category?: string;
  conditionId?: string;
  negRisk?: boolean;
  startDateIso?: string;
  startDate?: string;
  endDateIso?: string;
  endDate?: string;
  slug?: string;
  updatedAt?: string;
}

// ---------------------------------------------------------------------------
// Pricing types for pairs
// ---------------------------------------------------------------------------

interface OutcomePricing {
  bid: number | null;
  ask: number | null;
  price: number | null;
}

interface PairPricing {
  predict: {yes: OutcomePricing; no: OutcomePricing};
  polymarket: {yes: OutcomePricing; no: OutcomePricing};
}

interface TrimmedPredictMarket {
  id: string;
  question: string;
  category: MarketCategory;
  statistics: MarketStatistics;
  totalPositions: number;
  orderbook: OrderbookView | null;
  source: string;
  sourceUrl: string;
}

interface TrimmedPolymarketMarket {
  id: string;
  question: string;
  category: MarketCategory;
  statistics: MarketStatistics;
  orderbook: OrderbookView | null;
  orderbookTokens?: {tokenId: string; outcome: string}[];
  source: string;
  sourceUrl: string;
}

interface MarketPair {
  id: string;
  similarity: number;
  question: string;
  predict: TrimmedPredictMarket;
  polymarket: TrimmedPolymarketMarket;
  pricing: PairPricing;
}

interface ViewMarket {
  id: string;
  title: string;
  question: string;
  imageUrl: string;
  category: MarketCategory;
  chancePercentage: number | null;
  spreadThreshold: string;
  spreadDecimal: string | null;
  spreadPercent: string | null;
  makerFeeBps: number | null;
  takerFeeBps: number | null;
  isTradingEnabled: boolean | null;
  status: string | null;
  shareThreshold: string | null;
  statistics: MarketStatistics | null;
  outcomes: {id: string; name: string; index: number; positionsCount: number}[];
  totalPositions: number;
  source: string | null;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const sleep = (ms: number): Promise<void> =>
  new Promise((resolve) => setTimeout(resolve, ms));

const fetchJson = async (url: string, retries = 2): Promise<unknown> => {
  let attempt = 0;
  while (true) {
    try {
      const res = await fetch(url, {
        headers: {'User-Agent': 'predict-market'},
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(`HTTP ${res.status}: ${text}`);
      }
      return (await res.json()) as unknown;
    } catch (error) {
      attempt += 1;
      if (attempt > retries) {
        throw error;
      }
      await sleep(400 * attempt);
    }
  }
};

const parseJsonArray = (value: unknown): unknown[] => {
  if (Array.isArray(value)) return value;
  if (typeof value === 'string') {
    try {
      const parsed: unknown = JSON.parse(value);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }
  return [];
};

const toNumber = (value: unknown): number | null => {
  if (value === null || value === undefined || value === '') return null;
  const num = Number(value);
  return Number.isFinite(num) ? num : null;
};

const STOPWORDS: ReadonlySet<string> = new Set([
  'a',
  'an',
  'and',
  'are',
  'as',
  'at',
  'be',
  'before',
  'by',
  'does',
  'for',
  'from',
  'if',
  'in',
  'into',
  'is',
  'it',
  'of',
  'on',
  'or',
  'the',
  'this',
  'to',
  'was',
  'were',
  'will',
  'with',
]);

const tokenize = (text: string): string[] => {
  if (!text) return [];
  const cleaned = text
    .toLowerCase()
    .replace(/['']/g, '')
    .replace(/[^a-z0-9]+/g, ' ')
    .trim();

  return cleaned
    .split(/\s+/)
    .filter((token) => token.length > 2 || /^\d+$/.test(token))
    .filter((token) => !STOPWORDS.has(token));
};

const tokenSet = (text: string): Set<string> => new Set(tokenize(text));

const bigramSet = (text: string): Set<string> => {
  if (!text) return new Set();
  const cleaned = text
    .toLowerCase()
    .replace(/['']/g, '')
    .replace(/[^a-z0-9]+/g, '');
  if (cleaned.length < 2) return new Set();

  const grams = new Set<string>();
  for (let i = 0; i < cleaned.length - 1; i += 1) {
    grams.add(cleaned.slice(i, i + 2));
  }
  return grams;
};

const diceCoefficient = (aSet: Set<string>, bSet: Set<string>): number => {
  if (!aSet.size || !bSet.size) return 0;
  let overlap = 0;
  for (const token of aSet) {
    if (bSet.has(token)) overlap += 1;
  }
  return (2 * overlap) / (aSet.size + bSet.size);
};

const extractNumbers = (text: string): Set<string> => {
  if (!text) return new Set();
  const matches = text.match(/\d+(\.\d+)?/g) ?? [];
  return new Set(matches);
};

const extractYears = (text: string): Set<string> => {
  const years = new Set<string>();
  for (const value of extractNumbers(text)) {
    if (value.length !== 4) continue;
    const year = Number(value);
    if (Number.isFinite(year) && year >= 1900 && year <= 2100) {
      years.add(value);
    }
  }
  return years;
};

const hasOverlap = (aSet: Set<string>, bSet: Set<string>): boolean => {
  for (const value of aSet) {
    if (bSet.has(value)) return true;
  }
  return false;
};

const normalizeOutcomeName = (name: string | null | undefined): string =>
  (name ?? '')
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '')
    .trim();

const YES_NAMES: ReadonlySet<string> = new Set(['yes', 'true', 'y', '1']);
const NO_NAMES: ReadonlySet<string> = new Set(['no', 'false', 'n', '0']);

interface YesNoOutcomes {
  yes: MarketOutcome;
  no: MarketOutcome;
}

const extractYesNo = (
  outcomes: MarketOutcome[],
): YesNoOutcomes | null => {
  if (!Array.isArray(outcomes)) return null;
  const yes = outcomes.find((outcome) =>
    YES_NAMES.has(normalizeOutcomeName(outcome.name)),
  );
  const no = outcomes.find((outcome) =>
    NO_NAMES.has(normalizeOutcomeName(outcome.name)),
  );
  if (!yes || !no) return null;
  return {yes, no};
};

const buildPricing = (outcome: MarketOutcome | undefined): OutcomePricing => ({
  bid: toNumber(outcome?.bidPriceInCurrency ?? outcome?.bidPrice),
  ask: toNumber(outcome?.askPriceInCurrency ?? outcome?.askPrice),
  price: toNumber(outcome?.price),
});

const trimPredict = (market: NormalizedMarket): TrimmedPredictMarket => ({
  id: market.id,
  question: market.question,
  category: market.category,
  statistics: market.statistics,
  totalPositions: market.totalPositions,
  orderbook: market.orderbook,
  source: market.source,
  sourceUrl: market.sourceUrl,
});

const trimPolymarket = (
  market: NormalizedMarket,
): TrimmedPolymarketMarket => ({
  id: market.id,
  question: market.question,
  category: market.category,
  statistics: market.statistics,
  orderbook: market.orderbook,
  orderbookTokens: market.orderbookTokens,
  source: market.source,
  sourceUrl: market.sourceUrl,
});

// ---------------------------------------------------------------------------
// Predict.Fun fetch (delegates to fetch_open_markets.ts via tsx)
// ---------------------------------------------------------------------------

const fetchPredictMarkets = (argv: string[]): Promise<void> =>
  new Promise<void>((resolve, reject) => {
    const args = [
      path.join(currentDir, 'fetch_open_markets.ts'),
      '--full-out',
      PREDICT_FULL,
      '--out',
      PREDICT_VIEW,
      ...argv,
    ];

    const child: ChildProcess = spawn(
      path.join(currentDir, 'node_modules', '.bin', 'tsx'),
      args,
      {
        cwd: currentDir,
        stdio: ['ignore', 'pipe', 'pipe'],
      },
    );

    child.stdout?.on('data', (chunk: Buffer) => process.stdout.write(chunk));
    child.stderr?.on('data', (chunk: Buffer) => process.stderr.write(chunk));

    child.on('error', reject);
    child.on('close', (code: number | null) => {
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`fetch_open_markets.ts exited with ${code}`));
      }
    });
  });

const filterPredictArgs = (argv: string[]): string[] => {
  const filtered: string[] = [];
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg.startsWith('--full-out')) {
      if (!arg.includes('=')) i += 1;
      continue;
    }
    if (arg.startsWith('--out')) {
      if (!arg.includes('=')) i += 1;
      continue;
    }
    filtered.push(arg);
  }
  return filtered;
};

// ---------------------------------------------------------------------------
// Polymarket normalization & fetch
// ---------------------------------------------------------------------------

const normalizePolymarketMarket = (
  market: PolymarketRawMarket,
): NormalizedMarket => {
  const outcomeNames = parseJsonArray(market.outcomes) as string[];
  const outcomePrices = (
    parseJsonArray(market.outcomePrices) as (string | number)[]
  ).map(toNumber);
  const clobTokenIds = parseJsonArray(market.clobTokenIds) as string[];

  const outcomes: MarketOutcome[] = outcomeNames.map(
    (name: string, index: number) => ({
      id: `poly_${market.id}_${index}`,
      name,
      index,
      status: null,
      onChainId: clobTokenIds[index] ?? null,
      bidPriceInCurrency: null,
      askPriceInCurrency: null,
      sharesCount: null,
      positionsValueUsd: null,
      positionsCount: 0,
      price: outcomePrices[index] ?? null,
    }),
  );

  const bestAsk = toNumber(market.bestAsk);
  const bestBid = toNumber(market.bestBid);
  const spread =
    bestAsk !== null && bestBid !== null
      ? Number((bestAsk - bestBid).toFixed(6))
      : null;
  const spreadCents =
    spread !== null ? Number((spread * 100).toFixed(4)) : null;

  const liquidity =
    toNumber(market.liquidityNum ?? market.liquidity) ?? 0;
  const volumeTotal =
    toNumber(market.volumeNum ?? market.volume) ?? 0;
  const volume24h =
    toNumber(market.volume24hr ?? market.volume24hrClob) ?? 0;

  const priceCandidates = outcomePrices.filter(
    (value): value is number => Number.isFinite(value),
  );
  const chance =
    priceCandidates.length > 0 ? Math.max(...priceCandidates) : null;

  return {
    id: `poly_${market.id}`,
    title: market.question ?? 'Polymarket Market',
    question: market.question ?? 'Polymarket Market',
    description: market.description ?? null,
    imageUrl: market.image ?? market.icon ?? '',
    createdAt: market.createdAt ?? null,
    status: market.closed ? 'CLOSED' : 'OPEN',
    isTradingEnabled: market.acceptingOrders ?? false,
    chancePercentage:
      chance !== null ? Number((chance * 100).toFixed(2)) : null,
    spreadThreshold: market.spread ? String(market.spread) : '0',
    spreadThresholdDecimal: market.spread ? String(market.spread) : '0',
    spreadThresholdPercent:
      spreadCents !== null ? `${spreadCents.toFixed(2)}¢` : '-',
    shareThreshold: '',
    makerFeeBps: 0,
    takerFeeBps: 0,
    decimalPrecision: 6,
    oracleQuestionId: null,
    oracleTxHash: null,
    conditionId: market.conditionId ?? null,
    resolverAddress: null,
    questionIndex: null,
    category: {
      id: market.category ?? 'polymarket',
      title: market.category ?? 'Polymarket',
      description: market.description ?? null,
      imageUrl: market.image ?? market.icon ?? '',
      isNegRisk: market.negRisk ?? false,
      isYieldBearing: false,
      startsAt: market.startDateIso ?? market.startDate ?? null,
      endsAt: market.endDateIso ?? market.endDate ?? null,
      status: market.active ? 'ACTIVE' : 'INACTIVE',
      holdersCount: null,
      commentsTotal: null,
      tags: [],
    },
    statistics: {
      totalLiquidityUsd: liquidity,
      volumeTotalUsd: volumeTotal,
      volume24hUsd: volume24h,
      volume24hChangeUsd: 0,
      percentageChanceChange24h: null,
    },
    outcomes,
    totalPositions: 0,
    resolution: null,
    statusLogs: null,
    bulletinBoardUpdates: [],
    orderbook: {
      updateTimestampMs: market.updatedAt
        ? Date.parse(market.updatedAt)
        : null,
      orderCount: null,
      lastOrderSettled: market.lastTradePrice
        ? {price: String(market.lastTradePrice), side: ''}
        : null,
      bestAsk,
      bestBid,
      spread,
      spreadCents,
      asks: [],
      bids: [],
      settlementsPending: null,
    },
    holders: null,
    comments: null,
    timeseries: null,
    source: 'Polymarket',
    sourceUrl: market.slug
      ? `https://polymarket.com/market/${market.slug}`
      : 'https://polymarket.com/',
    orderbookTokens: clobTokenIds.map(
      (tokenId: string, index: number) => ({
        tokenId,
        outcome: outcomeNames[index] ?? `Outcome ${index + 1}`,
      }),
    ),
  };
};

const fetchPolymarketMarkets = async (): Promise<NormalizedMarket[]> => {
  const markets: NormalizedMarket[] = [];
  let offset = 0;

  while (true) {
    const params = new URLSearchParams({
      limit: String(POLYMARKET_PAGE_SIZE),
      offset: String(offset),
    });
    if (POLYMARKET_ACTIVE_ONLY) {
      params.set('active', 'true');
      params.set('closed', 'false');
    }

    const url = `${POLYMARKET_API}/markets?${params.toString()}`;
    const batch = (await fetchJson(url)) as PolymarketRawMarket[];
    if (!Array.isArray(batch) || batch.length === 0) {
      break;
    }

    for (const raw of batch) {
      if (POLYMARKET_ACCEPTING_ONLY && raw.acceptingOrders !== true) {
        continue;
      }
      markets.push(normalizePolymarketMarket(raw));
      if (
        POLYMARKET_MAX_MARKETS > 0 &&
        markets.length >= POLYMARKET_MAX_MARKETS
      ) {
        return markets;
      }
    }

    offset += batch.length;
    if (batch.length < POLYMARKET_PAGE_SIZE) {
      break;
    }
  }

  return markets;
};

// ---------------------------------------------------------------------------
// Pair matching
// ---------------------------------------------------------------------------

interface PolyIndexEntry {
  market: NormalizedMarket;
  tokens: Set<string>;
  bigrams: Set<string>;
  numbers: Set<string>;
  years: Set<string>;
  yesNo: YesNoOutcomes;
}

interface ScoredCandidate {
  score: number;
  entry: PolyIndexEntry;
}

const buildPairs = (
  predictMarkets: NormalizedMarket[],
  polymarketMarkets: NormalizedMarket[],
): MarketPair[] => {
  const polyIndex: PolyIndexEntry[] = polymarketMarkets
    .map((market): PolyIndexEntry | null => {
      const yesNo = extractYesNo(market.outcomes ?? []);
      if (!yesNo) return null;
      const question = market.question ?? '';
      const tokens = tokenSet(question);
      if (tokens.size < PAIR_MIN_TOKENS) return null;
      return {
        market,
        tokens,
        bigrams: bigramSet(question),
        numbers: extractNumbers(question),
        years: extractYears(question),
        yesNo,
      };
    })
    .filter((entry): entry is PolyIndexEntry => entry !== null);

  const usedPoly = new Set<string>();
  const pairs: MarketPair[] = [];

  for (const predict of predictMarkets) {
    const predictYesNo = extractYesNo(predict.outcomes ?? []);
    if (!predictYesNo) continue;
    const question = predict.question ?? '';
    const predictTokens = tokenSet(question);
    if (predictTokens.size < PAIR_MIN_TOKENS) continue;
    const predictBigrams = bigramSet(question);
    const predictNumbers = extractNumbers(question);
    const predictYears = extractYears(question);

    let best: ScoredCandidate | null = null;
    let second: ScoredCandidate | null = null;

    for (const entry of polyIndex) {
      if (usedPoly.has(entry.market.id)) continue;
      if (
        PAIR_REQUIRE_YEAR_MATCH &&
        predictYears.size &&
        entry.years.size &&
        !hasOverlap(predictYears, entry.years)
      ) {
        continue;
      }
      if (
        PAIR_REQUIRE_NUMBER_MATCH &&
        predictNumbers.size &&
        entry.numbers.size &&
        !hasOverlap(predictNumbers, entry.numbers)
      ) {
        continue;
      }

      const tokenScore = diceCoefficient(predictTokens, entry.tokens);
      if (tokenScore < PAIR_MIN_SIMILARITY) continue;
      const charScore = diceCoefficient(predictBigrams, entry.bigrams);
      if (charScore < PAIR_MIN_CHAR_SIMILARITY) continue;

      const score = (tokenScore + charScore) / 2;
      if (!best || score > best.score) {
        second = best;
        best = {score, entry};
      } else if (!second || score > second.score) {
        second = {score, entry};
      }
    }

    if (!best || best.score < PAIR_MIN_SIMILARITY) continue;
    if (second && best.score - second.score < PAIR_MIN_MARGIN) continue;

    usedPoly.add(best.entry.market.id);
    pairs.push({
      id: `pair_${predict.id}_${best.entry.market.id}`,
      similarity: Number(best.score.toFixed(4)),
      question: predict.question,
      predict: trimPredict(predict),
      polymarket: trimPolymarket(best.entry.market),
      pricing: {
        predict: {
          yes: buildPricing(predictYesNo.yes),
          no: buildPricing(predictYesNo.no),
        },
        polymarket: {
          yes: buildPricing(best.entry.yesNo.yes),
          no: buildPricing(best.entry.yesNo.no),
        },
      },
    });
  }

  return pairs;
};

// ---------------------------------------------------------------------------
// I/O
// ---------------------------------------------------------------------------

const writeJson = async (filePath: string, payload: unknown): Promise<void> => {
  await mkdir(path.dirname(filePath), {recursive: true});
  await writeFile(filePath, JSON.stringify(payload, null, 2), 'utf-8');
};

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

const main = async (): Promise<void> => {
  const predictArgs = filterPredictArgs(process.argv.slice(2));

  await fetchPredictMarkets(predictArgs);
  const predictPayload = JSON.parse(
    await readFile(PREDICT_FULL, 'utf-8'),
  ) as PredictFullPayload;
  const predictMarkets: NormalizedMarket[] = (
    predictPayload.markets ?? []
  ).map((market) => ({
    ...market,
    source: 'Predict.Fun',
    sourceUrl: 'https://predict.fun/',
  }));

  let polymarketMarkets: NormalizedMarket[] = [];
  try {
    polymarketMarkets = await fetchPolymarketMarkets();
  } catch (error) {
    const message =
      error instanceof Error ? error.message : String(error);
    console.error(`polymarket_error=${message}`);
  }

  const combined: NormalizedMarket[] = [
    ...predictMarkets,
    ...polymarketMarkets,
  ];
  const generatedAt = new Date().toISOString();

  await writeJson(OUT_FULL, {
    generatedAt,
    count: combined.length,
    markets: combined,
  });

  const viewMarkets: ViewMarket[] = combined.map((market) => ({
    id: market.id,
    title: market.title,
    question: market.question,
    imageUrl: market.imageUrl,
    category: market.category,
    chancePercentage: market.chancePercentage,
    spreadThreshold: market.spreadThreshold,
    spreadDecimal: market.spreadThresholdDecimal ?? null,
    spreadPercent: market.spreadThresholdPercent ?? null,
    makerFeeBps: market.makerFeeBps ?? null,
    takerFeeBps: market.takerFeeBps ?? null,
    isTradingEnabled: market.isTradingEnabled ?? null,
    status: market.status ?? null,
    shareThreshold: market.shareThreshold ?? null,
    statistics: market.statistics ?? null,
    outcomes: (market.outcomes ?? []).map((outcome) => ({
      id: outcome.id,
      name: outcome.name,
      index: outcome.index,
      positionsCount: outcome.positionsCount ?? 0,
    })),
    totalPositions: market.totalPositions ?? 0,
    source: market.source ?? null,
  }));

  await writeJson(OUT_VIEW, {
    generatedAt,
    count: combined.length,
    markets: viewMarkets,
  });

  const pairs = buildPairs(predictMarkets, polymarketMarkets);
  await writeJson(PAIRS_OUT, {
    generatedAt,
    count: pairs.length,
    pairs,
  });

  console.error(
    `combined=${combined.length} predict=${predictMarkets.length} polymarket=${polymarketMarkets.length} pairs=${pairs.length}`,
  );
};

await main();
