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
struct Variables {
    filter: Filter,
    sort: &'static str,
    pagination: Pagination,
}

#[derive(Serialize)]
struct Filter {
    #[serde(rename = "isResolved")]
    is_resolved: bool,
}

#[derive(Serialize)]
struct Pagination {
    first: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    after: Option<String>,
}

#[derive(Deserialize)]
struct GqlResponse {
    data: DataField,
}

#[derive(Deserialize)]
struct DataField {
    markets: MarketsField,
}

#[derive(Deserialize)]
struct MarketsField {
    #[serde(rename = "pageInfo")]
    page_info: PageInfo,
    edges: Vec<Edge>,
}

#[derive(Deserialize)]
struct PageInfo {
    #[serde(rename = "hasNextPage")]
    has_next_page: bool,
    #[serde(rename = "endCursor")]
    end_cursor: Option<String>,
}

#[derive(Deserialize)]
struct Edge {
    node: MarketNode,
}

#[derive(Deserialize)]
struct MarketNode {
    id: String,
    #[allow(dead_code)]
    status: String,
    #[allow(dead_code)]
    #[serde(rename = "isTradingEnabled")]
    is_trading_enabled: bool,
}

fn main() {
    let t0 = Instant::now();
    let client = Client::builder()
        .timeout(std::time::Duration::from_secs(30))
        .build()
        .expect("failed to build client");

    let mut markets: Vec<MarketNode> = Vec::new();
    let mut cursor: Option<String> = None;

    loop {
        let req = GqlRequest {
            operation_name: "GetMarkets",
            variables: Variables {
                filter: Filter { is_resolved: false },
                sort: "VOLUME_24H_DESC",
                pagination: Pagination {
                    first: 50,
                    after: cursor.clone(),
                },
            },
            query: QUERY,
        };

        let resp: GqlResponse = client
            .post(GRAPHQL_URL)
            .json(&req)
            .send()
            .expect("request failed")
            .json()
            .expect("json parse failed");

        for edge in resp.data.markets.edges {
            markets.push(edge.node);
        }

        if !resp.data.markets.page_info.has_next_page {
            break;
        }
        cursor = resp.data.markets.page_info.end_cursor;
    }

    let elapsed = t0.elapsed().as_secs_f64();
    println!("Rust: {} markets in {:.3}s", markets.len(), elapsed);
}
