/*
 * C++ benchmark: fetch all Predict.Fun open markets via GraphQL using libcurl.
 * Compile: g++ -std=c++17 -O2 -o bench_cpp bench.cpp -lcurl
 */
#include <iostream>
#include <iomanip>
#include <string>
#include <chrono>
#include <cstring>
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
    while ((pos = haystack.find(needle, pos)) != std::string::npos) {
        ++count;
        pos += needle.size();
    }
    return count;
}

int main() {
    auto t0 = std::chrono::steady_clock::now();

    curl_global_init(CURL_GLOBAL_DEFAULT);
    CURL *curl = curl_easy_init();
    if (!curl) { std::cerr << "curl init failed\n"; return 1; }

    curl_slist *headers = nullptr;
    headers = curl_slist_append(headers, "Content-Type: application/json");

    int total_markets = 0;
    std::string cursor;
    bool has_cursor = false;

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
        curl_easy_setopt(curl, CURLOPT_URL, GRAPHQL_URL);
        curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
        curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
        curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response);
        curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);

        CURLcode res = curl_easy_perform(curl);
        if (res != CURLE_OK) {
            std::cerr << "curl error: " << curl_easy_strerror(res) << "\n";
            break;
        }

        total_markets += count_occurrences(response, "\"isTradingEnabled\":");

        if (!has_next_page(response)) break;

        cursor = json_string_value(response, "endCursor");
        has_cursor = !cursor.empty();
        if (!has_cursor) break;
    }

    auto t1 = std::chrono::steady_clock::now();
    double elapsed = std::chrono::duration<double>(t1 - t0).count();
    std::cout << "C++: " << total_markets << " markets in "
              << std::fixed << std::setprecision(3) << elapsed << "s\n";

    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
    curl_global_cleanup();
    return 0;
}
