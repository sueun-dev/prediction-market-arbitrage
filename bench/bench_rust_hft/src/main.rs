use reqwest::blocking::Client;
use serde::{Deserialize, Serialize};
use std::time::Instant;

const GRAPHQL_URL: &str = "https://graphql.predict.fun/graphql";
const QUERY: &str = r#"query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {
  markets(filter: $filter, sort: $sort, pagination: $pagination) {
    pageInfo { hasNextPage endCursor }
    edges { node { id status isTradingEnabled category { id } } }
  }
}"#;

#[derive(Serialize)]
struct GqlRequest {
    #[serde(rename = "operationName")]
    operation_name: &'static str,
    variables: Variables,
    query: &'static str,
}
#[derive(Serialize)]
struct Variables { filter: Filter, sort: &'static str, pagination: Pagination }
#[derive(Serialize)]
struct Filter { #[serde(rename = "isResolved")] is_resolved: bool }
#[derive(Serialize)]
struct Pagination { first: u32, #[serde(skip_serializing_if = "Option::is_none")] after: Option<String> }
#[derive(Deserialize)]
struct GqlResponse { data: DataField }
#[derive(Deserialize)]
struct DataField { markets: MarketsField }
#[derive(Deserialize)]
struct MarketsField { #[serde(rename = "pageInfo")] page_info: PageInfo, edges: Vec<Edge> }
#[derive(Deserialize)]
struct PageInfo { #[serde(rename = "hasNextPage")] has_next_page: bool, #[serde(rename = "endCursor")] end_cursor: Option<String> }
#[derive(Deserialize)]
struct Edge { node: MarketNode }
#[derive(Deserialize)]
struct MarketNode {
    #[allow(dead_code)] id: String,
    #[allow(dead_code)] status: String,
    #[allow(dead_code)] #[serde(rename = "isTradingEnabled")] is_trading_enabled: bool,
}

#[derive(Serialize)]
struct BenchResult {
    lang: &'static str,
    markets: usize,
    #[serde(rename = "totalSec")] total_sec: f64,
    requests: usize,
    #[serde(rename = "firstDataMs")] first_data_ms: f64,
    #[serde(rename = "avgLatMs")] avg_lat_ms: f64,
    #[serde(rename = "minLatMs")] min_lat_ms: f64,
    #[serde(rename = "maxLatMs")] max_lat_ms: f64,
    #[serde(rename = "p50Ms")] p50_ms: f64,
    #[serde(rename = "p99Ms")] p99_ms: f64,
    #[serde(rename = "rssKb")] rss_kb: u64,
    #[serde(rename = "heapKb")] heap_kb: u64,
}

fn get_rss_kb() -> u64 {
    let usage = unsafe {
        let mut u: libc_rusage = std::mem::zeroed();
        libc_getrusage(0, &mut u);
        u
    };
    (usage.ru_maxrss as u64) / 1024 // macOS reports bytes
}

// Minimal libc bindings for getrusage
#[repr(C)]
struct libc_rusage {
    ru_utime: [i64; 2],
    ru_stime: [i64; 2],
    ru_maxrss: i64,
    _pad: [i64; 13],
}
extern "C" { fn getrusage(who: i32, usage: *mut libc_rusage) -> i32; }
unsafe fn libc_getrusage(who: i32, usage: &mut libc_rusage) {
    getrusage(who, usage as *mut libc_rusage);
}

fn main() {
    let t0 = Instant::now();
    let client = Client::builder()
        .timeout(std::time::Duration::from_secs(30))
        .tcp_keepalive(std::time::Duration::from_secs(60))
        .pool_max_idle_per_host(10)
        .build()
        .expect("client");

    let mut markets: Vec<MarketNode> = Vec::new();
    let mut cursor: Option<String> = None;
    let mut latencies: Vec<f64> = Vec::new();
    let mut first_data_ms: f64 = 0.0;

    loop {
        let req = GqlRequest {
            operation_name: "GetMarkets",
            variables: Variables {
                filter: Filter { is_resolved: false },
                sort: "VOLUME_24H_DESC",
                pagination: Pagination { first: 50, after: cursor.clone() },
            },
            query: QUERY,
        };

        let req_start = Instant::now();
        let resp: GqlResponse = client
            .post(GRAPHQL_URL)
            .json(&req)
            .send().expect("send")
            .json().expect("json");
        let lat_ms = req_start.elapsed().as_secs_f64() * 1000.0;
        latencies.push(lat_ms);

        if first_data_ms == 0.0 && !resp.data.markets.edges.is_empty() {
            first_data_ms = t0.elapsed().as_secs_f64() * 1000.0;
        }

        for edge in resp.data.markets.edges {
            markets.push(edge.node);
        }
        if !resp.data.markets.page_info.has_next_page { break; }
        cursor = resp.data.markets.page_info.end_cursor;
    }

    let elapsed = t0.elapsed().as_secs_f64();
    latencies.sort_by(|a, b| a.partial_cmp(b).unwrap());
    let n = latencies.len();
    let sum: f64 = latencies.iter().sum();

    let result = BenchResult {
        lang: "Rust",
        markets: markets.len(),
        total_sec: (elapsed * 1000.0).round() / 1000.0,
        requests: n,
        first_data_ms: (first_data_ms * 10.0).round() / 10.0,
        avg_lat_ms: ((sum / n as f64) * 10.0).round() / 10.0,
        min_lat_ms: (latencies[0] * 10.0).round() / 10.0,
        max_lat_ms: (latencies[n - 1] * 10.0).round() / 10.0,
        p50_ms: (latencies[n / 2] * 10.0).round() / 10.0,
        p99_ms: (latencies[(n as f64 * 0.99) as usize] * 10.0).round() / 10.0,
        rss_kb: get_rss_kb(),
        heap_kb: 0,
    };

    println!("{}", serde_json::to_string(&result).unwrap());
}
