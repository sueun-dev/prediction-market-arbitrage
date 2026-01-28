import { mkdir, writeFile } from "node:fs/promises";
import { dirname } from "node:path";

const GRAPHQL_URL = "https://graphql.predict.fun/graphql";
const WS_URL = "wss://ws.predict.fun/ws";
const WEI_DECIMALS = 18;

const GET_MARKETS_INDEX_QUERY = `
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
        id
        status
        isTradingEnabled
        category {
          id
        }
      }
    }
  }
}
`.trim();

const GET_MARKET_DETAIL_QUERY = `
query GetMarketFull($marketId: ID!) {
  market(id: $marketId) {
    id
    title
    question
    description
    imageUrl
    createdAt
    status
    isTradingEnabled
    chancePercentage
    spreadThreshold
    shareThreshold
    makerFeeBps
    takerFeeBps
    decimalPrecision
    oracleQuestionId
    oracleTxHash
    conditionId
    resolverAddress
    questionIndex
    category {
      id
      title
      description
      imageUrl
      isNegRisk
      isYieldBearing
      startsAt
      endsAt
      status
      holdersCount
      comments {
        totalCount
      }
      tags {
        edges {
          node {
            id
            name
          }
        }
      }
    }
    statistics {
      totalLiquidityUsd
      volumeTotalUsd
      volume24hUsd
      volume24hChangeUsd
      percentageChanceChange24h
    }
    outcomes {
      edges {
        node {
          id
          index
          name
          status
          onChainId
          bidPriceInCurrency
          askPriceInCurrency
          statistics {
            sharesCount
            positionsValueUsd
          }
          positions {
            totalCount
          }
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
    statusLogs {
      edges {
        node {
          status
          timestamp
          transactionHash
        }
      }
    }
    bulletinBoardUpdates {
      content
      createdAt
      updatedAt
      transactionHash
    }
  }
}
`.trim();

const GET_MARKET_HOLDERS_QUERY = `
query GetMarketHolders(
  $marketId: ID!
  $filter: OutcomeFilterInput
  $pagination: ForwardPaginationInput
) {
  market(id: $marketId) {
    outcomes(filter: $filter) {
      edges {
        node {
          id
          index
          name
          positions(pagination: $pagination) {
            pageInfo {
              hasNextPage
              startCursor
              endCursor
            }
            totalCount
            edges {
              node {
                id
                shares
                valueUsd
                account {
                  address
                  name
                }
              }
              cursor
            }
          }
        }
      }
    }
  }
}
`.trim();

const GET_COMMENTS_QUERY = `
query GetComments(
  $categoryId: ID!
  $onlyHolders: Boolean
  $pagination: ForwardPaginationInput
  $sortBy: CommentSortBy
  $repliesPagination: ForwardPaginationInput
) {
  comments(
    categoryId: $categoryId
    onlyHolders: $onlyHolders
    pagination: $pagination
    sortBy: $sortBy
  ) {
    pageInfo {
      hasNextPage
      startCursor
      endCursor
    }
    totalCount
    edges {
      ...CommentEdge
    }
  }
}

fragment CommentEdge on CommentEdge {
  cursor
  node {
    ...Comment
    replies(pagination: $repliesPagination) {
      pageInfo {
        hasNextPage
        endCursor
      }
      totalCount
      edges {
        node {
          ...Comment
        }
      }
    }
  }
}

fragment Comment on Comment {
  id
  content
  createdAt
  updatedAt
  likeCount
  isLikedByUser
  replyCount
  reportCount
  account {
    address
    name
    imageUrl
  }
  parentComment {
    id
  }
  replyToComment {
    id
    account {
      name
      address
    }
  }
}
`.trim();

const GET_CATEGORY_TIMESERIES_QUERY = `
query GetCategoryTimeseries(
  $categoryId: ID!
  $interval: TimeseriesInterval!
  $pagination: ForwardPaginationInput
) {
  category(id: $categoryId) {
    timeseries(filter: { interval: $interval }, pagination: $pagination) {
      pageInfo {
        hasNextPage
        endCursor
      }
      edges {
        node {
          dataGranularity
          market {
            id
          }
          data {
            edges {
              node {
                x
                y
              }
            }
          }
        }
      }
    }
  }
}
`.trim();

const LOAD_MORE_REPLIES_QUERY = `
query LoadMoreReplies($commentId: ID!, $after: String, $first: Int!) {
  comment(id: $commentId) {
    id
    replyCount
    replies(pagination: { first: $first, after: $after }) {
      pageInfo {
        hasNextPage
        endCursor
      }
      totalCount
      edges {
        node {
          id
          content
          createdAt
          updatedAt
          likeCount
          isLikedByUser
          replyCount
          reportCount
          account {
            address
            name
            imageUrl
          }
          parentComment {
            id
          }
          replyToComment {
            id
            account {
              name
              address
            }
          }
        }
      }
    }
  }
}
`.trim();

type MarketIndex = {
  id: string;
  status: string;
  isTradingEnabled: boolean;
  category: { id: string };
};

type MarketDetail = {
  id: string;
  title: string;
  question: string;
  description: string | null;
  imageUrl: string;
  createdAt: string;
  status: string;
  isTradingEnabled: boolean;
  chancePercentage: number;
  spreadThreshold: string;
  shareThreshold: string;
  makerFeeBps: number;
  takerFeeBps: number;
  decimalPrecision: number;
  oracleQuestionId: string | null;
  oracleTxHash: string | null;
  conditionId: string | null;
  resolverAddress: string | null;
  questionIndex: number | null;
  category: {
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
    comments: { totalCount: number | null } | null;
    tags: { edges: { node: { id: string; name: string } }[] } | null;
  };
  statistics: {
    totalLiquidityUsd: number;
    volumeTotalUsd: number;
    volume24hUsd: number;
    volume24hChangeUsd: number;
    percentageChanceChange24h: number;
  };
  outcomes: {
    edges: {
      node: {
        id: string;
        index: number;
        name: string;
        status: string | null;
        onChainId: string;
        bidPriceInCurrency: number | null;
        askPriceInCurrency: number | null;
        statistics: { sharesCount: number; positionsValueUsd: number } | null;
        positions: { totalCount: number };
      };
    }[];
  };
  resolution: {
    id: string;
    name: string;
    index: number;
    status: string;
    createdAt: string;
  } | null;
  statusLogs: {
    edges: {
      node: {
        status: string;
        timestamp: string;
        transactionHash: string | null;
      };
    }[];
  };
  bulletinBoardUpdates: {
    content: string;
    createdAt: string;
    updatedAt: string | null;
    transactionHash: string | null;
  }[];
};

type HolderPosition = {
  id: string;
  shares: string;
  valueUsd: number | null;
  account: { address: string; name: string | null };
};

type MarketHolders = {
  outcomes: {
    outcomeId: string;
    index: number;
    name: string;
    totalCount: number;
    positions: (HolderPosition & { sharesDecimal: string })[];
  }[];
};

type CommentNode = {
  id: string;
  content: string;
  createdAt: string;
  updatedAt: string | null;
  likeCount: number;
  isLikedByUser: boolean;
  replyCount: number;
  reportCount: number;
  account: { address: string; name: string | null; imageUrl: string | null };
  parentComment: { id: string } | null;
  replyToComment: { id: string; account: { name: string | null; address: string } } | null;
  replies?: {
    pageInfo: { hasNextPage: boolean; endCursor: string | null };
    totalCount: number | null;
    edges: { node: CommentNode }[];
  };
};

type CommentEdge = { cursor: string; node: CommentNode };

type CategoryComments = {
  totalCount: number;
  edges: CommentEdge[];
};

type TimeseriesPoint = { x: number; y: number };

type TimeseriesData = {
  dataGranularity: string;
  points: TimeseriesPoint[];
};

type TimeseriesInterval = "_1D" | "_7D" | "_30D" | "MAX";

type OrderbookSnapshot = {
  version: number;
  marketId: number;
  updateTimestampMs: number;
  lastOrderSettled?: {
    id: string;
    price: string;
    kind: string;
    side: string;
    outcome: string;
    marketId: number;
  };
  orderCount: number;
  asks: [number, number][];
  bids: [number, number][];
  settlementsPending?: number;
};

type OrderbookView = {
  updateTimestampMs: number;
  orderCount: number;
  lastOrderSettled: OrderbookSnapshot["lastOrderSettled"] | null;
  bestAsk: number | null;
  bestBid: number | null;
  spread: number | null;
  spreadCents: number | null;
  asks: { price: number; size: number }[];
  bids: { price: number; size: number }[];
  settlementsPending: number | null;
};

type MarketFullView = {
  id: string;
  title: string;
  question: string;
  description: string | null;
  imageUrl: string;
  createdAt: string;
  status: string;
  isTradingEnabled: boolean;
  chancePercentage: number;
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
  category: {
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
    tags: { id: string; name: string }[];
  };
  statistics: MarketDetail["statistics"];
  outcomes: {
    id: string;
    name: string;
    index: number;
    status: string | null;
    onChainId: string;
    bidPriceInCurrency: number | null;
    askPriceInCurrency: number | null;
    sharesCount: number | null;
    positionsValueUsd: number | null;
    positionsCount: number;
  }[];
  totalPositions: number;
  resolution: MarketDetail["resolution"];
  statusLogs: MarketDetail["statusLogs"];
  bulletinBoardUpdates: MarketDetail["bulletinBoardUpdates"];
  orderbook: OrderbookView | null;
  holders: MarketHolders | null;
  comments: CategoryComments | null;
  timeseries: Partial<Record<TimeseriesInterval, TimeseriesData>> | null;
};

type MarketIndexResponse = {
  data: {
    markets: {
      pageInfo: {
        hasNextPage: boolean;
        startCursor: string | null;
        endCursor: string | null;
      };
      edges: { node: MarketIndex }[];
    };
  };
  errors?: Array<{ message: string }>;
};

type MarketDetailResponse = {
  data: { market: MarketDetail };
  errors?: Array<{ message: string }>;
};

type MarketHoldersResponse = {
  data: {
    market: {
      outcomes: {
        edges: {
          node: {
            id: string;
            index: number;
            name: string;
            positions: {
              pageInfo: {
                hasNextPage: boolean;
                startCursor: string | null;
                endCursor: string | null;
              };
              totalCount: number;
              edges: { node: HolderPosition; cursor: string }[];
            };
          };
        }[];
      };
    };
  };
  errors?: Array<{ message: string }>;
};

type CommentsResponse = {
  data: {
    comments: {
      pageInfo: {
        hasNextPage: boolean;
        startCursor: string | null;
        endCursor: string | null;
      };
      totalCount: number;
      edges: CommentEdge[];
    };
  };
  errors?: Array<{ message: string }>;
};

type RepliesResponse = {
  data: {
    comment: {
      id: string;
      replyCount: number;
      replies: {
        pageInfo: { hasNextPage: boolean; endCursor: string | null };
        totalCount: number;
        edges: { node: CommentNode }[];
      };
    };
  };
  errors?: Array<{ message: string }>;
};

type TimeseriesResponse = {
  data: {
    category: {
      timeseries: {
        pageInfo: { hasNextPage: boolean; endCursor: string | null };
        edges: {
          node: {
            dataGranularity: string;
            market: { id: string };
            data: { edges: { node: TimeseriesPoint }[] };
          };
        }[];
      };
    };
  };
  errors?: Array<{ message: string }>;
};

type Config = {
  pageSize: number;
  sort: string;
  sleepSeconds: number;
  timeoutSeconds: number;
  retries: number;
  backoffSeconds: number;
  statusFilter: string;
  includeNonBettable: boolean;
  viewOutPath: string;
  fullOutPath: string;
  rawOutPath: string | null;
  includeOrderbook: boolean;
  includeHolders: boolean;
  includeComments: boolean;
  includeTimeseries: boolean;
  commentsLimit: number;
  holdersLimit: number;
  repliesLimit: number;
  fetchAllReplies: boolean;
  orderbookTimeoutMs: number;
  maxMarkets: number;
  concurrency: number;
  categoryConcurrency: number;
  timeseriesIntervals: TimeseriesInterval[];
  authToken: string | null;
  cookie: string | null;
};

const DEFAULTS: Config = {
  pageSize: 50,
  sort: "VOLUME_24H_DESC",
  sleepSeconds: 0,
  timeoutSeconds: 20,
  retries: 3,
  backoffSeconds: 0.5,
  statusFilter: "",
  includeNonBettable: false,
  viewOutPath: "site/data/markets_view.json",
  fullOutPath: "site/data/markets_full.json",
  rawOutPath: null,
  includeOrderbook: true,
  includeHolders: true,
  includeComments: true,
  includeTimeseries: true,
  commentsLimit: 0,
  holdersLimit: 0,
  repliesLimit: 20,
  fetchAllReplies: false,
  orderbookTimeoutMs: 8000,
  maxMarkets: 0,
  concurrency: 4,
  categoryConcurrency: 3,
  timeseriesIntervals: ["_1D", "_7D", "_30D", "MAX"],
  authToken: null,
  cookie: null,
};

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const toBigInt = (value: string) => BigInt(value || "0");

const formatDecimal = (
  value: string,
  decimals = WEI_DECIMALS,
  precision = 6,
) => {
  const negative = value.startsWith("-");
  const raw = negative ? value.slice(1) : value;
  const base = BigInt(10) ** BigInt(decimals);
  const num = toBigInt(raw);
  const intPart = num / base;
  const fracPart = (num % base).toString().padStart(decimals, "0");
  let frac = fracPart.slice(0, precision);
  frac = frac.replace(/0+$/, "");
  const out = frac.length ? `${intPart.toString()}.${frac}` : intPart.toString();
  return negative ? `-${out}` : out;
};

const formatPercent = (value: string, decimals = WEI_DECIMALS, precision = 2) => {
  const adjusted = Math.max(decimals - 2, 0);
  return `${formatDecimal(value, adjusted, precision)}%`;
};

const parseArgs = (argv: string[]): Config => {
  const config: Config = { ...DEFAULTS };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) {
      continue;
    }

    const [key, inlineValue] = arg.split("=");
    const value = inlineValue ?? argv[i + 1];

    switch (key) {
      case "--page-size":
        config.pageSize = Number(value ?? config.pageSize);
        if (!inlineValue) i += 1;
        break;
      case "--sort":
        config.sort = value ?? config.sort;
        if (!inlineValue) i += 1;
        break;
      case "--sleep":
        config.sleepSeconds = Number(value ?? config.sleepSeconds);
        if (!inlineValue) i += 1;
        break;
      case "--timeout":
        config.timeoutSeconds = Number(value ?? config.timeoutSeconds);
        if (!inlineValue) i += 1;
        break;
      case "--retries":
        config.retries = Number(value ?? config.retries);
        if (!inlineValue) i += 1;
        break;
      case "--backoff":
        config.backoffSeconds = Number(value ?? config.backoffSeconds);
        if (!inlineValue) i += 1;
        break;
      case "--status":
        config.statusFilter = value ?? config.statusFilter;
        if (!inlineValue) i += 1;
        break;
      case "--include-nonbettable":
        config.includeNonBettable = true;
        break;
      case "--out":
        config.viewOutPath = value ?? config.viewOutPath;
        if (!inlineValue) i += 1;
        break;
      case "--full-out":
        config.fullOutPath = value ?? config.fullOutPath;
        if (!inlineValue) i += 1;
        break;
      case "--raw-out":
        config.rawOutPath = value ?? config.rawOutPath;
        if (!inlineValue) i += 1;
        break;
      case "--skip-orderbook":
        config.includeOrderbook = false;
        break;
      case "--skip-holders":
        config.includeHolders = false;
        break;
      case "--skip-comments":
        config.includeComments = false;
        break;
      case "--skip-timeseries":
        config.includeTimeseries = false;
        break;
      case "--comments-limit":
        config.commentsLimit = Number(value ?? config.commentsLimit);
        if (!inlineValue) i += 1;
        break;
      case "--holders-limit":
        config.holdersLimit = Number(value ?? config.holdersLimit);
        if (!inlineValue) i += 1;
        break;
      case "--replies-limit":
        config.repliesLimit = Number(value ?? config.repliesLimit);
        if (!inlineValue) i += 1;
        break;
      case "--fetch-all-replies":
        config.fetchAllReplies = true;
        break;
      case "--orderbook-timeout":
        config.orderbookTimeoutMs = Number(value ?? config.orderbookTimeoutMs);
        if (!inlineValue) i += 1;
        break;
      case "--max-markets":
        config.maxMarkets = Number(value ?? config.maxMarkets);
        if (!inlineValue) i += 1;
        break;
      case "--concurrency":
        config.concurrency = Number(value ?? config.concurrency);
        if (!inlineValue) i += 1;
        break;
      case "--category-concurrency":
        config.categoryConcurrency = Number(value ?? config.categoryConcurrency);
        if (!inlineValue) i += 1;
        break;
      case "--timeseries-intervals":
        config.timeseriesIntervals = (value ?? "")
          .split(",")
          .map((entry) => entry.trim())
          .filter((entry) => entry.length > 0) as TimeseriesInterval[];
        if (!inlineValue) i += 1;
        break;
      case "--auth":
        config.authToken = value ?? null;
        if (!inlineValue) i += 1;
        break;
      case "--cookie":
        config.cookie = value ?? null;
        if (!inlineValue) i += 1;
        break;
      default:
        break;
    }
  }

  return config;
};

const buildHeaders = (config: Config) => {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  if (config.authToken) {
    headers.Authorization = `Bearer ${config.authToken}`;
  }

  if (config.cookie) {
    headers.Cookie = config.cookie;
  }

  return headers;
};

const postGraphql = async <T>(
  payload: Record<string, unknown>,
  config: Config,
): Promise<T> => {
  let attempt = 0;
  while (true) {
    try {
      const controller = new AbortController();
      const timeout = setTimeout(
        () => controller.abort(),
        config.timeoutSeconds * 1000,
      );

      const res = await fetch(GRAPHQL_URL, {
        method: "POST",
        headers: buildHeaders(config),
        body: JSON.stringify(payload),
        signal: controller.signal,
      });

      clearTimeout(timeout);

      if (!res.ok) {
        const message = await res.text();
        throw new Error(`HTTP ${res.status}: ${message}`);
      }

      const data = (await res.json()) as T;
      if ((data as { errors?: Array<{ message: string }> }).errors?.length) {
        throw new Error(
          JSON.stringify((data as { errors?: Array<{ message: string }> }).errors),
        );
      }
      return data;
    } catch (error) {
      attempt += 1;
      if (attempt > config.retries) {
        throw error;
      }
      await sleep(config.backoffSeconds * attempt * 1000);
    }
  }
};

const withConcurrency = async <T, R>(
  items: T[],
  limit: number,
  task: (item: T, index: number) => Promise<R>,
): Promise<R[]> => {
  const results: R[] = new Array(items.length);
  let index = 0;

  const workers = Array.from({ length: Math.max(1, limit) }, async () => {
    while (index < items.length) {
      const current = index;
      index += 1;
      results[current] = await task(items[current], current);
    }
  });

  await Promise.all(workers);
  return results;
};

const fetchMarketIndex = async (config: Config): Promise<MarketIndex[]> => {
  const markets: MarketIndex[] = [];
  let cursor: string | null = null;

  while (true) {
    const pagination: Record<string, unknown> = { first: config.pageSize };
    if (cursor) pagination.after = cursor;

    const payload = {
      operationName: "GetMarkets",
      variables: {
        filter: { isResolved: false },
        sort: config.sort,
        pagination,
      },
      query: GET_MARKETS_INDEX_QUERY,
    };

    const response = await postGraphql<MarketIndexResponse>(payload, config);
    const page = response.data.markets;
    for (const edge of page.edges) {
      markets.push(edge.node);
    }

    if (!page.pageInfo.hasNextPage) {
      break;
    }

    cursor = page.pageInfo.endCursor;
    if (config.sleepSeconds > 0) {
      await sleep(config.sleepSeconds * 1000);
    }
  }

  return markets;
};

const fetchMarketDetail = async (
  marketId: string,
  config: Config,
): Promise<MarketDetail> => {
  const payload = {
    operationName: "GetMarketFull",
    variables: { marketId },
    query: GET_MARKET_DETAIL_QUERY,
  };

  const response = await postGraphql<MarketDetailResponse>(payload, config);
  return response.data.market;
};

const fetchOutcomeHolders = async (
  marketId: string,
  outcomeIndex: number,
  config: Config,
): Promise<{ totalCount: number; positions: HolderPosition[] }> => {
  const positions: HolderPosition[] = [];
  let cursor: string | null = null;
  let totalCount = 0;

  while (true) {
    const pagination: Record<string, unknown> = { first: 50 };
    if (cursor) pagination.after = cursor;

    const payload = {
      operationName: "GetMarketHolders",
      variables: {
        marketId,
        filter: { outcomeIndex },
        pagination,
      },
      query: GET_MARKET_HOLDERS_QUERY,
    };

    const response = await postGraphql<MarketHoldersResponse>(payload, config);
    const outcomes = response.data.market.outcomes.edges;
    const outcome = outcomes[0]?.node;
    if (!outcome) {
      break;
    }

    totalCount = outcome.positions.totalCount ?? totalCount;

    for (const edge of outcome.positions.edges) {
      positions.push(edge.node);
      if (config.holdersLimit > 0 && positions.length >= config.holdersLimit) {
        return { totalCount, positions };
      }
    }

    if (!outcome.positions.pageInfo.hasNextPage) {
      break;
    }

    cursor = outcome.positions.pageInfo.endCursor;
  }

  return { totalCount, positions };
};

const fetchMarketHolders = async (
  market: MarketDetail,
  config: Config,
): Promise<MarketHolders> => {
  const outcomes = market.outcomes.edges.map((edge) => edge.node);
  const holders = [] as MarketHolders["outcomes"];

  for (const outcome of outcomes) {
    const { totalCount, positions } = await fetchOutcomeHolders(
      market.id,
      outcome.index,
      config,
    );

    holders.push({
      outcomeId: outcome.id,
      index: outcome.index,
      name: outcome.name,
      totalCount,
      positions: positions.map((position) => ({
        ...position,
        sharesDecimal: formatDecimal(position.shares),
      })),
    });
  }

  return { outcomes: holders };
};

const fetchComments = async (
  categoryId: string,
  config: Config,
): Promise<CategoryComments> => {
  const edges: CommentEdge[] = [];
  let cursor: string | null = null;
  let totalCount = 0;

  while (true) {
    const pagination: Record<string, unknown> = { first: 30 };
    if (cursor) pagination.after = cursor;

    const payload = {
      operationName: "GetComments",
      variables: {
        categoryId,
        onlyHolders: false,
        pagination,
        sortBy: "NEWEST",
        repliesPagination: { first: config.repliesLimit },
      },
      query: GET_COMMENTS_QUERY,
    };

    const response = await postGraphql<CommentsResponse>(payload, config);
    totalCount = response.data.comments.totalCount;
    for (const edge of response.data.comments.edges) {
      edges.push(edge);
      if (config.commentsLimit > 0 && edges.length >= config.commentsLimit) {
        return { totalCount, edges };
      }
    }

    if (!response.data.comments.pageInfo.hasNextPage) {
      break;
    }
    cursor = response.data.comments.pageInfo.endCursor;
  }

  if (config.fetchAllReplies) {
    for (const edge of edges) {
      if (!edge.node.replies?.pageInfo.hasNextPage) {
        continue;
      }
      const replyEdges = [...(edge.node.replies?.edges ?? [])];
      let replyCursor = edge.node.replies?.pageInfo.endCursor ?? null;
      while (replyCursor) {
        const payload = {
          operationName: "LoadMoreReplies",
          variables: {
            commentId: edge.node.id,
            after: replyCursor,
            first: config.repliesLimit,
          },
          query: LOAD_MORE_REPLIES_QUERY,
        };

        const response = await postGraphql<RepliesResponse>(payload, config);
        for (const reply of response.data.comment.replies.edges) {
          replyEdges.push(reply);
        }
        if (!response.data.comment.replies.pageInfo.hasNextPage) {
          replyCursor = null;
        } else {
          replyCursor = response.data.comment.replies.pageInfo.endCursor;
        }
      }
      if (edge.node.replies) {
        edge.node.replies.edges = replyEdges;
        edge.node.replies.pageInfo.hasNextPage = false;
      }
    }
  }

  return { totalCount, edges };
};

const fetchCategoryTimeseries = async (
  categoryId: string,
  interval: TimeseriesInterval,
  config: Config,
): Promise<Map<string, TimeseriesData>> => {
  const result = new Map<string, TimeseriesData>();
  let cursor: string | null = null;

  while (true) {
    const pagination: Record<string, unknown> = { first: 50 };
    if (cursor) pagination.after = cursor;

    const payload = {
      operationName: "GetCategoryTimeseries",
      variables: { categoryId, interval, pagination },
      query: GET_CATEGORY_TIMESERIES_QUERY,
    };

    const response = await postGraphql<TimeseriesResponse>(payload, config);
    for (const edge of response.data.category.timeseries.edges) {
      const points = edge.node.data.edges.map((entry) => entry.node);
      result.set(edge.node.market.id, {
        dataGranularity: edge.node.dataGranularity,
        points,
      });
    }

    if (!response.data.category.timeseries.pageInfo.hasNextPage) {
      break;
    }
    cursor = response.data.category.timeseries.pageInfo.endCursor;
  }

  return result;
};

const fetchOrderbookSnapshot = async (
  marketId: string,
  timeoutMs: number,
): Promise<OrderbookSnapshot | null> =>
  new Promise((resolve) => {
    let resolved = false;
    const ws = new WebSocket(WS_URL);
    const topic = `predictOrderbook/${marketId}`;
    const timeout = setTimeout(() => {
      if (!resolved) {
        resolved = true;
        ws.close();
        resolve(null);
      }
    }, timeoutMs);

    ws.onopen = () => {
      ws.send(
        JSON.stringify({
          requestId: 1,
          method: "subscribe",
          params: [topic],
        }),
      );
    };

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data as string) as {
          type: string;
          topic?: string;
          data?: OrderbookSnapshot;
        };
        if (message.type === "M" && message.topic === topic && message.data) {
          if (!resolved) {
            resolved = true;
            clearTimeout(timeout);
            ws.close();
            resolve(message.data);
          }
        }
      } catch {
        // Ignore parse errors and keep waiting.
      }
    };

    ws.onerror = () => {
      if (!resolved) {
        resolved = true;
        clearTimeout(timeout);
        resolve(null);
      }
    };

    ws.onclose = () => {
      if (!resolved) {
        resolved = true;
        clearTimeout(timeout);
        resolve(null);
      }
    };
  });

const normalizeOrderbook = (snapshot: OrderbookSnapshot | null): OrderbookView | null => {
  if (!snapshot) return null;
  const asks = snapshot.asks.map(([price, size]) => ({ price, size }));
  const bids = snapshot.bids.map(([price, size]) => ({ price, size }));
  const bestAsk = asks.length ? asks[0].price : null;
  const bestBid = bids.length ? bids[0].price : null;
  const spread =
    bestAsk !== null && bestBid !== null
      ? Number((bestAsk - bestBid).toFixed(6))
      : null;
  const spreadCents = spread !== null ? Number((spread * 100).toFixed(4)) : null;

  return {
    updateTimestampMs: snapshot.updateTimestampMs,
    orderCount: snapshot.orderCount,
    lastOrderSettled: snapshot.lastOrderSettled ?? null,
    bestAsk,
    bestBid,
    spread,
    spreadCents,
    asks,
    bids,
    settlementsPending: snapshot.settlementsPending ?? null,
  };
};

const toFullView = (
  market: MarketDetail,
  orderbook: OrderbookView | null,
  holders: MarketHolders | null,
  comments: CategoryComments | null,
  timeseries: Partial<Record<TimeseriesInterval, TimeseriesData>> | null,
): MarketFullView => {
  const outcomes = market.outcomes.edges.map((edge) => ({
    id: edge.node.id,
    name: edge.node.name,
    index: edge.node.index,
    status: edge.node.status,
    onChainId: edge.node.onChainId,
    bidPriceInCurrency: edge.node.bidPriceInCurrency,
    askPriceInCurrency: edge.node.askPriceInCurrency,
    sharesCount: edge.node.statistics?.sharesCount ?? null,
    positionsValueUsd: edge.node.statistics?.positionsValueUsd ?? null,
    positionsCount: edge.node.positions?.totalCount ?? 0,
  }));

  const totalPositions = outcomes.reduce(
    (sum, outcome) => sum + outcome.positionsCount,
    0,
  );

  return {
    id: market.id,
    title: market.title,
    question: market.question,
    description: market.description,
    imageUrl: market.imageUrl,
    createdAt: market.createdAt,
    status: market.status,
    isTradingEnabled: market.isTradingEnabled,
    chancePercentage: market.chancePercentage,
    spreadThreshold: market.spreadThreshold,
    spreadThresholdDecimal: formatDecimal(market.spreadThreshold),
    spreadThresholdPercent: formatPercent(market.spreadThreshold),
    shareThreshold: market.shareThreshold,
    makerFeeBps: market.makerFeeBps,
    takerFeeBps: market.takerFeeBps,
    decimalPrecision: market.decimalPrecision,
    oracleQuestionId: market.oracleQuestionId,
    oracleTxHash: market.oracleTxHash,
    conditionId: market.conditionId,
    resolverAddress: market.resolverAddress,
    questionIndex: market.questionIndex,
    category: {
      id: market.category.id,
      title: market.category.title,
      description: market.category.description,
      imageUrl: market.category.imageUrl,
      isNegRisk: market.category.isNegRisk,
      isYieldBearing: market.category.isYieldBearing,
      startsAt: market.category.startsAt,
      endsAt: market.category.endsAt,
      status: market.category.status,
      holdersCount: market.category.holdersCount,
      commentsTotal: market.category.comments?.totalCount ?? null,
      tags: market.category.tags?.edges.map((edge) => edge.node) ?? [],
    },
    statistics: market.statistics,
    outcomes,
    totalPositions,
    resolution: market.resolution,
    statusLogs: market.statusLogs,
    bulletinBoardUpdates: market.bulletinBoardUpdates,
    orderbook,
    holders,
    comments,
    timeseries,
  };
};

const toView = (market: MarketFullView) => ({
  id: market.id,
  title: market.title,
  question: market.question,
  imageUrl: market.imageUrl,
  category: market.category,
  chancePercentage: market.chancePercentage,
  spreadThreshold: market.spreadThreshold,
  spreadDecimal: market.spreadThresholdDecimal,
  spreadPercent: market.spreadThresholdPercent,
  makerFeeBps: market.makerFeeBps,
  takerFeeBps: market.takerFeeBps,
  isTradingEnabled: market.isTradingEnabled,
  status: market.status,
  shareThreshold: market.shareThreshold,
  statistics: market.statistics,
  outcomes: market.outcomes.map((outcome) => ({
    id: outcome.id,
    name: outcome.name,
    index: outcome.index,
    positionsCount: outcome.positionsCount,
  })),
  totalPositions: market.totalPositions,
});

const writeJson = async (path: string, payload: unknown) => {
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, JSON.stringify(payload, null, 2), "utf-8");
};

const main = async () => {
  const config = parseArgs(process.argv.slice(2));

  const indexMarkets = await fetchMarketIndex(config);
  const filtered = config.includeNonBettable
    ? indexMarkets
    : indexMarkets.filter((market) => {
        if (!market.isTradingEnabled) return false;
        if (config.statusFilter && market.status !== config.statusFilter) {
          return false;
        }
        return true;
      });

  const limited =
    config.maxMarkets > 0 ? filtered.slice(0, config.maxMarkets) : filtered;

  const categoryIds = Array.from(
    new Set(limited.map((market) => market.category.id)),
  );

  const commentsByCategory = new Map<string, CategoryComments>();
  if (config.includeComments) {
    await withConcurrency(categoryIds, config.categoryConcurrency, async (id) => {
      const comments = await fetchComments(id, config);
      commentsByCategory.set(id, comments);
      return null;
    });
  }

  const timeseriesByCategory = new Map<
    string,
    Map<string, Partial<Record<TimeseriesInterval, TimeseriesData>>>
  >();

  if (config.includeTimeseries) {
    await withConcurrency(categoryIds, config.categoryConcurrency, async (id) => {
      const marketMap = new Map<
        string,
        Partial<Record<TimeseriesInterval, TimeseriesData>>
      >();

      for (const interval of config.timeseriesIntervals) {
        const dataByMarket = await fetchCategoryTimeseries(id, interval, config);
        for (const [marketId, data] of dataByMarket.entries()) {
          const existing = marketMap.get(marketId) ?? {};
          existing[interval] = data;
          marketMap.set(marketId, existing);
        }
      }

      timeseriesByCategory.set(id, marketMap);
      return null;
    });
  }

  const fullMarkets = await withConcurrency(
    limited,
    config.concurrency,
    async (marketIndex) => {
      const detail = await fetchMarketDetail(marketIndex.id, config);

      const orderbookSnapshot = config.includeOrderbook
        ? await fetchOrderbookSnapshot(marketIndex.id, config.orderbookTimeoutMs)
        : null;
      const orderbook = normalizeOrderbook(orderbookSnapshot);

      const holders = config.includeHolders
        ? await fetchMarketHolders(detail, config)
        : null;

      const comments = commentsByCategory.get(detail.category.id) ?? null;

      const timeseries =
        timeseriesByCategory.get(detail.category.id)?.get(detail.id) ?? null;

      return toFullView(detail, orderbook, holders, comments, timeseries);
    },
  );

  const output = {
    generatedAt: new Date().toISOString(),
    count: fullMarkets.length,
    markets: fullMarkets,
  };

  await writeJson(config.fullOutPath, output);

  const viewPayload = {
    generatedAt: output.generatedAt,
    count: fullMarkets.length,
    markets: fullMarkets.map(toView),
  };
  await writeJson(config.viewOutPath, viewPayload);

  if (config.rawOutPath) {
    await writeJson(config.rawOutPath, {
      generatedAt: output.generatedAt,
      count: limited.length,
      markets: limited,
    });
  }

  const statusLabel = config.statusFilter || "ANY";
  console.error(
    `fetched=${fullMarkets.length} sort=${config.sort} status=${statusLabel} fullOut=${config.fullOutPath}`,
  );
};

await main();
