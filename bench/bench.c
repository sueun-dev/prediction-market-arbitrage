/*
 * C benchmark: fetch all Predict.Fun open markets via GraphQL using libcurl.
 * Compile: gcc -O2 -o bench_c bench.c -lcurl
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <curl/curl.h>

#define GRAPHQL_URL "https://graphql.predict.fun/graphql"
#define QUERY \
  "query GetMarkets($filter: MarketFilterInput, $sort: MarketSortInput, $pagination: ForwardPaginationInput) {" \
  "  markets(filter: $filter, sort: $sort, pagination: $pagination) {" \
  "    pageInfo { hasNextPage endCursor }" \
  "    edges { node { id status isTradingEnabled category { id } } }" \
  "  }" \
  "}"

typedef struct {
    char *data;
    size_t len;
    size_t cap;
} Buffer;

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

/* Minimal JSON string extraction: find "key":"value" and return value (caller frees). */
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

/* Check if "hasNextPage":true exists after a given position */
static int has_next_page(const char *json) {
    return strstr(json, "\"hasNextPage\":true") != NULL;
}

/* Count occurrences of a substring */
static int count_substr(const char *haystack, const char *needle) {
    int count = 0;
    const char *p = haystack;
    size_t nlen = strlen(needle);
    while ((p = strstr(p, needle)) != NULL) {
        count++;
        p += nlen;
    }
    return count;
}

int main(void) {
    struct timespec t0, t1;
    clock_gettime(CLOCK_MONOTONIC, &t0);

    curl_global_init(CURL_GLOBAL_DEFAULT);
    CURL *curl = curl_easy_init();
    if (!curl) { fprintf(stderr, "curl init failed\n"); return 1; }

    struct curl_slist *headers = NULL;
    headers = curl_slist_append(headers, "Content-Type: application/json");

    Buffer buf = { .data = malloc(4096), .len = 0, .cap = 4096 };

    int total_markets = 0;
    char *cursor = NULL;

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

        CURLcode res = curl_easy_perform(curl);
        if (res != CURLE_OK) {
            fprintf(stderr, "curl error: %s\n", curl_easy_strerror(res));
            break;
        }

        /* Count market nodes by counting "\"id\":" within edges */
        int page_count = count_substr(buf.data, "\"isTradingEnabled\":");
        total_markets += page_count;

        if (!has_next_page(buf.data)) break;

        free(cursor);
        cursor = json_string(buf.data, "endCursor");
        if (!cursor) break;
    }

    clock_gettime(CLOCK_MONOTONIC, &t1);
    double elapsed = (t1.tv_sec - t0.tv_sec) + (t1.tv_nsec - t0.tv_nsec) / 1e9;
    printf("C: %d markets in %.3fs\n", total_markets, elapsed);

    free(cursor);
    free(buf.data);
    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
    curl_global_cleanup();
    return 0;
}
