const primaryDataUrl = "/api/markets";
const fallbackDataUrl = "./data/markets_pairs.json";
const streamUrl = "/api/stream";
const fallbackPollMs = 120000;
const defaultPageSize = 60;

const formatNumber = (value) =>
  new Intl.NumberFormat("en-US").format(value ?? 0);

const formatCompactUsd = (value) =>
  new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    notation: "compact",
    maximumFractionDigits: 2,
  }).format(value ?? 0);

const formatDate = (value) => {
  if (!value) return "-";
  return new Date(value).toLocaleDateString();
};

const formatPercent = (value, digits = 1) => {
  if (value === null || value === undefined || Number.isNaN(value)) return "-";
  return `${Number(value).toFixed(digits)}%`;
};

const formatSignedPercent = (value, digits = 2) => {
  if (value === null || value === undefined || Number.isNaN(value)) return "-";
  const sign = value > 0 ? "+" : "";
  return `${sign}${Number(value).toFixed(digits)}%`;
};

const formatPrice = (value) => {
  if (value === null || value === undefined || Number.isNaN(value)) return "-";
  return `${(Number(value) * 100).toFixed(1)}¢`;
};

const spreadValue = (market) => {
  if (market.orderbook?.spreadCents != null) return market.orderbook.spreadCents;
  const parsed = Number.parseFloat(market.spreadThresholdPercent ?? "0");
  return Number.isFinite(parsed) ? parsed : 0;
};

const state = {
  query: "",
  category: "all",
  sort: "match_desc",
  visibleCount: defaultPageSize,
};

const grid = document.querySelector("#market-grid");
const searchInput = document.querySelector("#search");
const categorySelect = document.querySelector("#category");
const sortSelect = document.querySelector("#sort");
const statMarkets = document.querySelector("#stat-markets");
const statBets = document.querySelector("#stat-bets");
const statSpread = document.querySelector("#stat-spread");
const generatedAt = document.querySelector("#generated-at");
const liveDot = document.querySelector("#live-dot");
const streamStatus = document.querySelector("#stream-status");
const listCount = document.querySelector("#list-count");
const loadMoreBtn = document.querySelector("#load-more");

let allPairs = [];
let refreshInFlight = false;
let queuedRefresh = false;
let pollTimer = null;
let streamSource = null;
let reconnectTimer = null;
let pendingVisibilityRefresh = false;

const setStreamStatus = (stateValue, label) => {
  if (!streamStatus || !liveDot) return;
  const next = stateValue ?? "offline";
  const text =
    label ??
    {
      live: "Live",
      stale: "Updating",
      offline: "Offline",
      error: "Error",
    }[next] ??
    "Offline";

  streamStatus.textContent = text;
  streamStatus.className = `status ${next}`;
  liveDot.className = `live-dot ${next}`;
};

const fetchJson = async (url) => {
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`Failed to load data: ${res.status}`);
  }
  return res.json();
};

const loadData = async () => {
  try {
    return await fetchJson(primaryDataUrl);
  } catch (error) {
    return fetchJson(fallbackDataUrl);
  }
};

const updateStats = (pairs) => {
  const totalBets = pairs.reduce(
    (sum, pair) => sum + (pair.predict?.totalPositions ?? 0),
    0,
  );
  const avgMatch = pairs.length
    ? pairs.reduce((sum, pair) => sum + (pair.similarity ?? 0), 0) /
      pairs.length
    : 0;

  statMarkets.textContent = formatNumber(pairs.length);
  statBets.textContent = formatNumber(totalBets);
  statSpread.textContent = `${(avgMatch * 100).toFixed(1)}%`;
};

const buildDistribution = (outcomes) => {
  const outcomeValue = (outcome) => {
    const raw =
      typeof outcome.price === "number" ? outcome.price : outcome.positionsCount;
    return Number.isFinite(raw) ? raw : 0;
  };

  const total = outcomes.reduce(
    (sum, outcome) => sum + outcomeValue(outcome),
    0,
  );
  if (!total) {
    return "<div class=\"distribution\"></div>";
  }

  const segments = outcomes
    .map((outcome) => {
      const pct = Math.max(1, (outcomeValue(outcome) / total) * 100);
      return `<span style=\"width:${pct.toFixed(2)}%\"></span>`;
    })
    .join("");

  return `<div class=\"distribution\">${segments}</div>`;
};

const renderOrderbookList = (rows) => {
  if (!rows?.length) {
    return "<div class=\"empty\">No orderbook data</div>";
  }

  return rows
    .map(
      (row) =>
        `<div class=\"order-row\"><span>${formatPrice(row.price)}</span><strong>${formatNumber(
          row.size,
        )}</strong></div>`,
    )
    .join("");
};

const getCategoryTitle = (category) => {
  if (!category) return null;
  if (typeof category === "string") return category;
  return category.title ?? null;
};

const formatPricingMain = (pricing) => {
  if (!pricing) return "-";
  if (pricing.ask != null) return formatPrice(pricing.ask);
  if (pricing.price != null) return formatPrice(pricing.price);
  if (pricing.bid != null) return formatPrice(pricing.bid);
  return "-";
};

const formatPricingDetail = (pricing) => {
  if (!pricing) return "-";
  const parts = [];
  if (pricing.ask != null) parts.push(`ask ${formatPrice(pricing.ask)}`);
  if (pricing.bid != null) parts.push(`bid ${formatPrice(pricing.bid)}`);
  if (!parts.length && pricing.price != null) {
    parts.push(`mid ${formatPrice(pricing.price)}`);
  }
  return parts.length ? parts.join(" · ") : "-";
};

const renderPriceRow = (label, pricing) => `
  <div class="price-row">
    <span>${label}</span>
    <strong>${formatPricingMain(pricing)}</strong>
    <em>${formatPricingDetail(pricing)}</em>
  </div>
`;

const renderPolymarketOrderbook = (market) => {
  const tokens = (market.orderbookTokens ?? []).filter((token) => token.tokenId);
  if (!tokens.length) {
    return "<div class=\"empty\">No orderbook tokens</div>";
  }

  const payload = encodeURIComponent(JSON.stringify(tokens));
  const panels = tokens
    .map(
      (token) => `
        <div class="orderbook-panel" data-token-id="${token.tokenId}">
          <h5>${token.outcome ?? "Outcome"}</h5>
          <div class="orderbook-subgrid">
            <div>
              <h6>Asks</h6>
              <div class="orderbook-list placeholder" data-side="asks">
                Open to load
              </div>
            </div>
            <div>
              <h6>Bids</h6>
              <div class="orderbook-list placeholder" data-side="bids">
                Open to load
              </div>
            </div>
          </div>
        </div>
      `,
    )
    .join("");

  return `<div class="orderbook-grid polymarket" data-orderbook="${payload}">${panels}</div>`;
};

const renderHolders = (holders) => {
  if (!holders?.outcomes?.length) {
    return "<div class=\"empty\">No holders data</div>";
  }

  return holders.outcomes
    .map((outcome) => {
      const top = outcome.positions.slice(0, 4);
      const items = top
        .map(
          (position) =>
            `<div class=\"holder-row\"><span>${
              position.account.name ?? position.account.address.slice(0, 6)
            }</span><strong>${position.sharesDecimal}</strong></div>`,
        )
        .join("");
      return `
        <div class="detail-block">
          <h4>${outcome.name} Holders (${formatNumber(outcome.totalCount)})</h4>
          ${items || '<div class="empty">No holders yet</div>'}
        </div>
      `;
    })
    .join("");
};

const renderComments = (comments) => {
  if (!comments?.edges?.length) {
    return "<div class=\"empty\">No comments</div>";
  }

  return comments.edges
    .slice(0, 3)
    .map(
      (edge) =>
        `<div class=\"comment\"><strong>${
          edge.node.account.name ?? edge.node.account.address.slice(0, 6)
        }</strong><span>${edge.node.content}</span></div>`,
    )
    .join("");
};

const renderPredictOrderbookDetails = (market) => {
  const orderbook = market.orderbook ?? null;
  const spreadDisplay =
    orderbook?.spreadCents != null
      ? `${orderbook.spreadCents.toFixed(2)}¢`
      : "-";
  const lastTrade = orderbook?.lastOrderSettled?.price
    ? formatPrice(Number(orderbook.lastOrderSettled.price))
    : "-";
  const lastSide = orderbook?.lastOrderSettled?.side ?? "";

  const askRows = renderOrderbookList(orderbook?.asks?.slice(0, 8));
  const bidRows = renderOrderbookList(orderbook?.bids?.slice(0, 8));

  return `
    <div class="detail-block">
      <h4>Predict.Fun Orderbook</h4>
      <div class="detail-metric">
        <span>Best Ask</span>
        <strong>${formatPrice(orderbook?.bestAsk)}</strong>
      </div>
      <div class="detail-metric">
        <span>Best Bid</span>
        <strong>${formatPrice(orderbook?.bestBid)}</strong>
      </div>
      <div class="detail-metric">
        <span>Spread</span>
        <strong>${spreadDisplay}</strong>
      </div>
      <div class="detail-metric">
        <span>Last Trade</span>
        <strong>${lastTrade} ${lastSide}</strong>
      </div>
      <div class="orderbook-grid">
        <div>
          <h5>Asks</h5>
          ${askRows}
        </div>
        <div>
          <h5>Bids</h5>
          ${bidRows}
        </div>
      </div>
    </div>
  `;
};

const renderPair = (pair, index) => {
  const predict = pair.predict ?? {};
  const polymarket = pair.polymarket ?? {};
  const matchScore =
    pair.similarity != null ? `${(pair.similarity * 100).toFixed(1)}%` : "-";
  const predictCategory = getCategoryTitle(predict.category) ?? "Predict.Fun";
  const polyCategory = getCategoryTitle(polymarket.category) ?? "Polymarket";
  const predictPricing = pair.pricing?.predict ?? {};
  const polyPricing = pair.pricing?.polymarket ?? {};

  return `
    <article class="card pair-card" style="animation-delay:${index * 20}ms">
      <div class="pair-header">
        <div class="pair-title">
          <div class="source-label predict">Predict.Fun</div>
          <h2>${predict.question ?? pair.question ?? "Market Pair"}</h2>
          <p class="pair-subtitle">Polymarket: ${polymarket.question ?? "-"}</p>
        </div>
        <div class="pair-score">Match ${matchScore}</div>
      </div>
      <div class="pair-grid">
        <div class="platform-block predict">
          <div class="platform-header">
            <span>Predict.Fun</span>
            <a href="${predict.sourceUrl ?? "https://predict.fun/"}" target="_blank" rel="noopener">Open</a>
          </div>
          <div class="platform-meta">${predictCategory}</div>
          ${renderPriceRow("Yes", predictPricing.yes)}
          ${renderPriceRow("No", predictPricing.no)}
          <div class="platform-foot">
            <span>24h Vol</span>
            <strong>${formatCompactUsd(predict.statistics?.volume24hUsd)}</strong>
          </div>
        </div>
        <div class="platform-block polymarket">
          <div class="platform-header">
            <span>Polymarket</span>
            <a href="${polymarket.sourceUrl ?? "https://polymarket.com/"}" target="_blank" rel="noopener">Open</a>
          </div>
          <div class="platform-meta">${polyCategory}</div>
          ${renderPriceRow("Yes", polyPricing.yes)}
          ${renderPriceRow("No", polyPricing.no)}
          <div class="platform-foot">
            <span>24h Vol</span>
            <strong>${formatCompactUsd(polymarket.statistics?.volume24hUsd)}</strong>
          </div>
        </div>
      </div>

      <details class="details pair-details" data-source="Polymarket">
        <summary>Orderbooks</summary>
        <div class="details-grid">
          ${renderPredictOrderbookDetails(predict)}
          <div class="detail-block">
            <h4>Polymarket Orderbook</h4>
            ${renderPolymarketOrderbook(polymarket)}
          </div>
        </div>
      </details>
    </article>
  `;
};

const applyFilters = (pairs) => {
  const query = state.query.trim().toLowerCase();

  return pairs
    .filter((pair) => {
      const predictQuestion = pair.predict?.question ?? "";
      const polyQuestion = pair.polymarket?.question ?? "";
      const predictCategory = getCategoryTitle(pair.predict?.category) ?? "";
      const polyCategory = getCategoryTitle(pair.polymarket?.category) ?? "";
      const matchesQuery =
        !query ||
        predictQuestion.toLowerCase().includes(query) ||
        polyQuestion.toLowerCase().includes(query) ||
        predictCategory.toLowerCase().includes(query) ||
        polyCategory.toLowerCase().includes(query);
      const matchesCategory =
        state.category === "all" ||
        predictCategory === state.category ||
        polyCategory === state.category;
      return matchesQuery && matchesCategory;
    })
    .sort((a, b) => {
      switch (state.sort) {
        case "match_desc":
          return (b.similarity ?? 0) - (a.similarity ?? 0);
        case "volume24_desc":
          return (b.predict?.statistics?.volume24hUsd ?? 0) -
            (a.predict?.statistics?.volume24hUsd ?? 0);
        case "poly_volume24_desc":
          return (b.polymarket?.statistics?.volume24hUsd ?? 0) -
            (a.polymarket?.statistics?.volume24hUsd ?? 0);
        case "positions_desc":
        default:
          return (b.predict?.totalPositions ?? 0) -
            (a.predict?.totalPositions ?? 0);
      }
    });
};

const render = () => {
  const list = applyFilters(allPairs);
  const visibleCount = Math.min(state.visibleCount, list.length);
  const visible = list.slice(0, visibleCount);
  grid.innerHTML = visible.map(renderPair).join("");
  updateStats(list);
  updatePagination(visibleCount, list.length);
  attachPolymarketDetails();
};

const updatePagination = (visible, total) => {
  if (!listCount || !loadMoreBtn) return;
  listCount.textContent = `${formatNumber(visible)} / ${formatNumber(total)}`;
  const done = visible >= total;
  loadMoreBtn.disabled = done;
  loadMoreBtn.textContent = done ? "All loaded" : "Load more";
};

const loadPolymarketOrderbook = async (container) => {
  if (container.dataset.loading === "true") return;
  const loadedAt = Number(container.dataset.loadedAt ?? 0);
  if (loadedAt && Date.now() - loadedAt < 30000) return;

  const tokens = container.dataset.orderbook
    ? JSON.parse(decodeURIComponent(container.dataset.orderbook))
    : [];
  const tokenIds = tokens.map((token) => token.tokenId).filter(Boolean);

  if (!tokenIds.length) return;

  container.dataset.loading = "true";
  container.querySelectorAll(".orderbook-list").forEach((list) => {
    list.textContent = "Loading...";
    list.classList.add("placeholder");
  });

  try {
    const res = await fetch(
      `/api/polymarket/orderbook?token_ids=${tokenIds.join(",")}`,
      { cache: "no-store" },
    );
    if (!res.ok) {
      throw new Error(`Orderbook error: ${res.status}`);
    }
    const payload = await res.json();
    const lookup = new Map(
      (payload.data ?? []).map((entry) => [entry.tokenId, entry]),
    );

    container.querySelectorAll(".orderbook-panel").forEach((panel) => {
      const tokenId = panel.dataset.tokenId;
      const entry = lookup.get(tokenId);
      const asksTarget = panel.querySelector('[data-side="asks"]');
      const bidsTarget = panel.querySelector('[data-side="bids"]');

      if (!entry || !entry.ok) {
        const message = entry?.error ?? "Failed to load";
        if (asksTarget) asksTarget.textContent = message;
        if (bidsTarget) bidsTarget.textContent = message;
        return;
      }

      const asks = renderOrderbookList(entry.orderbook?.asks ?? []);
      const bids = renderOrderbookList(entry.orderbook?.bids ?? []);
      if (asksTarget) {
        asksTarget.classList.remove("placeholder");
        asksTarget.innerHTML = asks;
      }
      if (bidsTarget) {
        bidsTarget.classList.remove("placeholder");
        bidsTarget.innerHTML = bids;
      }
    });

    container.dataset.loadedAt = String(Date.now());
  } catch (error) {
    container.querySelectorAll(".orderbook-list").forEach((list) => {
      list.textContent = error.message;
    });
  } finally {
    container.dataset.loading = "false";
  }
};

const attachPolymarketDetails = () => {
  grid.querySelectorAll('details[data-source="Polymarket"]').forEach(
    (details) => {
      details.addEventListener("toggle", () => {
        if (!details.open) return;
        const container = details.querySelector(".orderbook-grid.polymarket");
        if (container) {
          loadPolymarketOrderbook(container);
        }
      });
    },
  );
};

const updateCategories = (pairs) => {
  const categories = Array.from(
    new Set(
      pairs
        .flatMap((pair) => [
          getCategoryTitle(pair.predict?.category),
          getCategoryTitle(pair.polymarket?.category),
        ])
        .filter(Boolean),
    ),
  ).sort();

  categorySelect.innerHTML = [
    '<option value="all">All Categories</option>',
    ...categories.map(
      (category) => `<option value="${category}">${category}</option>`,
    ),
  ].join("");

  if (state.category !== "all" && !categories.includes(state.category)) {
    state.category = "all";
  }
  categorySelect.value = state.category;
};

const applyData = (data) => {
  allPairs = data.pairs ?? data.markets ?? [];
  updateCategories(allPairs);

  if (data.generatedAt) {
    generatedAt.textContent = new Date(data.generatedAt).toLocaleString();
  }

  render();
};

const refreshData = async () => {
  if (refreshInFlight) {
    queuedRefresh = true;
    return;
  }

  if (document.hidden && allPairs.length) {
    pendingVisibilityRefresh = true;
    return;
  }

  refreshInFlight = true;
  setStreamStatus("stale", "Updating");

  try {
    const data = await loadData();
    applyData(data);
    setStreamStatus("live");
  } catch (error) {
    if (!allPairs.length) {
      grid.innerHTML = `<p>Failed to load data. ${error.message}</p>`;
    }
    setStreamStatus("error", "Error");
  } finally {
    refreshInFlight = false;
    if (queuedRefresh) {
      queuedRefresh = false;
      refreshData();
    }
  }
};

const startPolling = () => {
  if (pollTimer) return;
  pollTimer = setInterval(refreshData, fallbackPollMs);
};

const stopPolling = () => {
  if (!pollTimer) return;
  clearInterval(pollTimer);
  pollTimer = null;
};

const connectStream = () => {
  if (!("EventSource" in window)) {
    startPolling();
    return;
  }
  if (streamSource) return;

  const source = new EventSource(streamUrl);
  streamSource = source;

  source.addEventListener("open", () => {
    stopPolling();
    setStreamStatus("live");
  });

  source.addEventListener("hello", () => {
    stopPolling();
    refreshData();
  });

  source.addEventListener("update", () => {
    stopPolling();
    refreshData();
  });

  source.addEventListener("status", (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload.state === "updating") {
        setStreamStatus("stale", "Updating");
      }
      if (payload.state === "error") {
        setStreamStatus("error", "Error");
      }
    } catch {
      // ignore payload parsing errors
    }
  });

  source.onerror = () => {
    source.close();
    streamSource = null;
    setStreamStatus("offline");
    startPolling();
    if (!reconnectTimer) {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connectStream();
      }, 15000);
    }
  };
};

const init = () => {
  setStreamStatus("offline");

  let searchTimer = null;
  searchInput.addEventListener("input", (event) => {
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      state.query = event.target.value;
      state.visibleCount = defaultPageSize;
      render();
    }, 120);
  });

  categorySelect.addEventListener("change", (event) => {
    state.category = event.target.value;
    state.visibleCount = defaultPageSize;
    render();
  });

  sortSelect.addEventListener("change", (event) => {
    state.sort = event.target.value;
    state.visibleCount = defaultPageSize;
    render();
  });

  if (loadMoreBtn) {
    loadMoreBtn.addEventListener("click", () => {
      state.visibleCount += defaultPageSize;
      render();
    });
  }

  document.addEventListener("visibilitychange", () => {
    if (!document.hidden && pendingVisibilityRefresh) {
      pendingVisibilityRefresh = false;
      refreshData();
    }
  });

  refreshData();
  connectStream();
};

init();
