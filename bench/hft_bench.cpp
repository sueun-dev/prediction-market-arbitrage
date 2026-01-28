/*
 * HFT C++ benchmark with per-request latency, memory, connection reuse.
 * Compile: g++ -std=c++17 -O2 -o hft_bench_cpp hft_bench.cpp -lcurl
 */
#include <iostream>
#include <iomanip>
#include <string>
#include <vector>
#include <chrono>
#include <algorithm>
#include <numeric>
#include <cstring>
#include <sys/resource.h>
#include <curl/curl.h>

static const char *GRAPHQL_URL = "https://graphql.predict.fun/graphql";
static const char *QUERY =
  "query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {"
  "  markets(filter: $filter, sort: $sort, pagination: $pagination) {"
  "    pageInfo { hasNextPage endCursor }"
  "    edges { node { id status isTradingEnabled category { id } } }"
  "  }"
  "}";

static size_t write_cb(void *ptr, size_t size, size_t nmemb, void *userdata) {
    auto *buf = static_cast<std::string *>(userdata);
    buf->append(static_cast<char *>(ptr), size * nmemb);
    return size * nmemb;
}

static std::string json_string_value(const std::string &json, const std::string &key) {
    std::string pattern = "\"" + key + "\":\"";
    auto pos = json.find(pattern);
    if (pos == std::string::npos) return "";
    pos += pattern.size();
    auto end = json.find('"', pos);
    if (end == std::string::npos) return "";
    return json.substr(pos, end - pos);
}

static bool has_next_page(const std::string &json) {
    return json.find("\"hasNextPage\":true") != std::string::npos;
}

static int count_occurrences(const std::string &haystack, const std::string &needle) {
    int count = 0;
    std::string::size_type pos = 0;
    while ((pos = haystack.find(needle, pos)) != std::string::npos) { ++count; pos += needle.size(); }
    return count;
}

using Clock = std::chrono::steady_clock;

int main() {
    auto t0 = Clock::now();

    curl_global_init(CURL_GLOBAL_DEFAULT);
    CURL *curl = curl_easy_init();
    curl_slist *headers = nullptr;
    headers = curl_slist_append(headers, "Content-Type: application/json");
    curl_easy_setopt(curl, CURLOPT_TCP_KEEPALIVE, 1L);
    curl_easy_setopt(curl, CURLOPT_TCP_KEEPIDLE, 60L);

    std::vector<double> latencies;
    int total_markets = 0;
    std::string cursor;
    bool has_cursor = false;
    double first_data_ms = 0;

    while (true) {
        std::string body;
        if (has_cursor) {
            body = std::string("{\"operationName\":\"GetMarkets\",")
                + "\"variables\":{\"filter\":{\"isResolved\":false},\"sort\":\"VOLUME_24H_DESC\","
                + "\"pagination\":{\"first\":50,\"after\":\"" + cursor + "\"}},"
                + "\"query\":\"" + QUERY + "\"}";
        } else {
            body = std::string("{\"operationName\":\"GetMarkets\",")
                + "\"variables\":{\"filter\":{\"isResolved\":false},\"sort\":\"VOLUME_24H_DESC\","
                + "\"pagination\":{\"first\":50}},"
                + "\"query\":\"" + QUERY + "\"}";
        }

        std::string response;
        response.reserve(65536);
        curl_easy_setopt(curl, CURLOPT_URL, GRAPHQL_URL);
        curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
        curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
        curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response);
        curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);

        auto req_start = Clock::now();
        CURLcode res = curl_easy_perform(curl);
        auto req_end = Clock::now();

        if (res != CURLE_OK) { std::cerr << "curl: " << curl_easy_strerror(res) << "\n"; break; }

        double lat_ms = std::chrono::duration<double, std::milli>(req_end - req_start).count();
        latencies.push_back(lat_ms);

        int page_count = count_occurrences(response, "\"isTradingEnabled\":");
        total_markets += page_count;

        if (!first_data_ms && page_count > 0) {
            first_data_ms = std::chrono::duration<double, std::milli>(req_end - t0).count();
        }

        if (!has_next_page(response)) break;
        cursor = json_string_value(response, "endCursor");
        has_cursor = !cursor.empty();
        if (!has_cursor) break;
    }

    double elapsed = std::chrono::duration<double>(Clock::now() - t0).count();

    std::sort(latencies.begin(), latencies.end());
    double sum = std::accumulate(latencies.begin(), latencies.end(), 0.0);
    size_t n = latencies.size();

    struct rusage usage;
    getrusage(RUSAGE_SELF, &usage);
    long rss_kb = usage.ru_maxrss / 1024;

    std::cout << std::fixed << std::setprecision(1);
    std::cout << "{\"lang\":\"C++\",\"markets\":" << total_markets
              << ",\"totalSec\":" << std::setprecision(3) << elapsed
              << ",\"requests\":" << n
              << ",\"firstDataMs\":" << std::setprecision(1) << first_data_ms
              << ",\"avgLatMs\":" << sum/n
              << ",\"minLatMs\":" << latencies.front()
              << ",\"maxLatMs\":" << latencies.back()
              << ",\"p50Ms\":" << latencies[n/2]
              << ",\"p99Ms\":" << latencies[(size_t)(n*0.99)]
              << ",\"rssKb\":" << rss_kb
              << ",\"heapKb\":0}\n";

    curl_slist_free_all(headers); curl_easy_cleanup(curl); curl_global_cleanup();
    return 0;
}
