export interface NormalizedOrderbookRow {
  price: number;
  size: number;
}

export interface NormalizedOrderbook {
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

export interface PolymarketBookPayload {
  timestamp?: number | string;
  error?: string;
  asks?: {price: string; size: string}[];
  bids?: {price: string; size: string}[];
}

const parseTimestamp = (
  value: number | string | undefined,
): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
};

const normalizeRows = (
  rows: {price: string; size: string}[],
  isAsk: boolean,
  levels: number,
): NormalizedOrderbookRow[] => {
  const valid = rows
    .map((row): NormalizedOrderbookRow => ({
      price: Number(row.price),
      size: Number(row.size),
    }))
    .filter(
      (row) =>
        Number.isFinite(row.price) &&
        Number.isFinite(row.size) &&
        row.price > 0 &&
        row.price <= 1 &&
        row.size > 0,
    )
    .sort((left, right) => {
      if (left.price === right.price) {
        return right.size - left.size;
      }
      return isAsk ? left.price - right.price : right.price - left.price;
    });

  if (levels > 0) {
    return valid.slice(0, levels);
  }
  return valid;
};

export const normalizePolymarketBook = (
  payload: PolymarketBookPayload,
  levels: number,
): NormalizedOrderbook => {
  const asks = normalizeRows(payload.asks ?? [], true, levels);
  const bids = normalizeRows(payload.bids ?? [], false, levels);

  const bestAsk = asks.length ? asks[0].price : null;
  const bestBid = bids.length ? bids[0].price : null;
  const spread =
    bestAsk !== null && bestBid !== null
      ? Number((bestAsk - bestBid).toFixed(6))
      : null;
  const spreadCents =
    spread !== null ? Number((spread * 100).toFixed(4)) : null;

  return {
    updateTimestampMs: parseTimestamp(payload.timestamp),
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
