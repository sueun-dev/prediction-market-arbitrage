/*
 * HFT C benchmark with per-request latency, memory, connection reuse.
 * Compile: gcc -O2 -o hft_bench_c hft_bench.c -lcurl -lm
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <time.h>
#include <sys/resource.h>
#include <curl/curl.h>

#define GRAPHQL_URL "https://graphql.predict.fun/graphql"
#define QUERY \
  "query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {" \
  "  markets(filter: $filter, sort: $sort, pagination: $pagination) {" \
  "    pageInfo { hasNextPage endCursor }" \
  "    edges { node { id status isTradingEnabled category { id } } }" \
  "  }" \
  "}"
#define MAX_REQUESTS 200

typedef struct { char *data; size_t len; size_t cap; } Buffer;

static size_t write_cb(void *ptr, size_t size, size_t nmemb, void *userdata) {
    Buffer *buf = (Buffer *)userdata;
    size_t bytes = size * nmemb;
    if (buf->len + bytes + 1 > buf->cap) {
        buf->cap = (buf->len + bytes + 1) * 2;
        buf->data = realloc(buf->data, buf->cap);
    }
    memcpy(buf->data + buf->len, ptr, bytes);
    buf->len += bytes;
    buf->data[buf->len] = '\0';
    return bytes;
}

static double now_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1000.0 + ts.tv_nsec / 1e6;
}

static char *json_string(const char *json, const char *key) {
    char pattern[256];
    snprintf(pattern, sizeof(pattern), "\"%s\":\"", key);
    const char *start = strstr(json, pattern);
    if (!start) return NULL;
    start += strlen(pattern);
    const char *end = strchr(start, '"');
    if (!end) return NULL;
    size_t len = (size_t)(end - start);
    char *val = malloc(len + 1);
    memcpy(val, start, len);
    val[len] = '\0';
    return val;
}

static int has_next_page(const char *json) {
    return strstr(json, "\"hasNextPage\":true") != NULL;
}

static int count_substr(const char *haystack, const char *needle) {
    int count = 0; const char *p = haystack; size_t nlen = strlen(needle);
    while ((p = strstr(p, needle)) != NULL) { count++; p += nlen; }
    return count;
}

static int cmp_double(const void *a, const void *b) {
    double da = *(const double *)a, db = *(const double *)b;
    return (da > db) - (da < db);
}

int main(void) {
    double t0 = now_ms();

    curl_global_init(CURL_GLOBAL_DEFAULT);
    CURL *curl = curl_easy_init();
    struct curl_slist *headers = NULL;
    headers = curl_slist_append(headers, "Content-Type: application/json");

    /* Enable connection reuse explicitly */
    curl_easy_setopt(curl, CURLOPT_TCP_KEEPALIVE, 1L);
    curl_easy_setopt(curl, CURLOPT_TCP_KEEPIDLE, 60L);

    Buffer buf = { .data = malloc(65536), .len = 0, .cap = 65536 };
    double latencies[MAX_REQUESTS];
    int req_count = 0, total_markets = 0;
    char *cursor = NULL;
    double first_data_ms = 0;

    while (1) {
        char body[2048];
        if (cursor) {
            snprintf(body, sizeof(body),
                "{\"operationName\":\"GetMarkets\","
                "\"variables\":{\"filter\":{\"isResolved\":false},\"sort\":\"VOLUME_24H_DESC\","
                "\"pagination\":{\"first\":50,\"after\":\"%s\"}},"
                "\"query\":\"%s\"}", cursor, QUERY);
        } else {
            snprintf(body, sizeof(body),
                "{\"operationName\":\"GetMarkets\","
                "\"variables\":{\"filter\":{\"isResolved\":false},\"sort\":\"VOLUME_24H_DESC\","
                "\"pagination\":{\"first\":50}},"
                "\"query\":\"%s\"}", QUERY);
        }

        buf.len = 0;
        curl_easy_setopt(curl, CURLOPT_URL, GRAPHQL_URL);
        curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body);
        curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
        curl_easy_setopt(curl, CURLOPT_WRITEDATA, &buf);
        curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);

        double req_start = now_ms();
        CURLcode res = curl_easy_perform(curl);
        double req_end = now_ms();

        if (res != CURLE_OK) { fprintf(stderr, "curl: %s\n", curl_easy_strerror(res)); break; }

        if (req_count < MAX_REQUESTS) latencies[req_count] = req_end - req_start;
        req_count++;

        int page_count = count_substr(buf.data, "\"isTradingEnabled\":");
        total_markets += page_count;

        if (!first_data_ms && page_count > 0) first_data_ms = req_end - t0;
        if (!has_next_page(buf.data)) break;

        free(cursor);
        cursor = json_string(buf.data, "endCursor");
        if (!cursor) break;
    }

    double elapsed = (now_ms() - t0) / 1000.0;

    int n = req_count < MAX_REQUESTS ? req_count : MAX_REQUESTS;
    qsort(latencies, n, sizeof(double), cmp_double);
    double sum = 0; for (int i = 0; i < n; i++) sum += latencies[i];

    struct rusage usage;
    getrusage(RUSAGE_SELF, &usage);
    long rss_kb = usage.ru_maxrss / 1024; /* macOS reports bytes */

    printf("{\"lang\":\"C\",\"markets\":%d,\"totalSec\":%.3f,"
           "\"requests\":%d,\"firstDataMs\":%.1f,"
           "\"avgLatMs\":%.1f,\"minLatMs\":%.1f,\"maxLatMs\":%.1f,"
           "\"p50Ms\":%.1f,\"p99Ms\":%.1f,"
           "\"rssKb\":%ld,\"heapKb\":0}\n",
           total_markets, elapsed, req_count, first_data_ms,
           sum/n, latencies[0], latencies[n-1],
           latencies[n/2], latencies[(int)(n*0.99)],
           rss_kb);

    free(cursor); free(buf.data);
    curl_slist_free_all(headers); curl_easy_cleanup(curl); curl_global_cleanup();
    return 0;
}
