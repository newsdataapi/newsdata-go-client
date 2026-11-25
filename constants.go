package newsdataapi

import (
	"net/http"
	"time"
)

// API root. The endpoint paths in `endpoints` carry the API version, so v1 and
// v2 endpoints share one base.
const baseURL = "https://newsdata.io/api/"

// Real-time WebSocket defaults (see WebSocket / NewWebSocket).
const (
	wsBaseURL = "wss://ws.newsdata.io/ws/event"
	// wsNewsType is the feed a registered query matches against; only news
	// so far.
	wsNewsType = "news"
	// wsPolicyViolation is the close code the server uses for a permanent
	// rejection (bad key, unknown registration_id, quota exhausted, ...).
	wsPolicyViolation          = 1008
	defaultWSReconnectDelay    = time.Second
	defaultWSReconnectDelayMax = 30 * time.Second
	defaultWSHandshakeTimeout  = 10 * time.Second
	defaultWSPingInterval      = 20 * time.Second
	defaultWSPongTimeout       = 20 * time.Second
)

// HTTP defaults.
const (
	defaultRequestTimeout  = 30 * time.Second
	defaultMaxRetries      = 5
	defaultRetryBackoff    = 2 * time.Second
	defaultRetryBackoffMax = 60 * time.Second
	defaultPaginationDelay = time.Second
)

// Response-size bounds. The API caps a single response at 50.
const (
	sizeMin = 1
	sizeMax = 50
)

// Endpoint key → path appended to baseURL. The key is also used to look up the
// allowed-parameter set in `filters`.
var endpoints = map[string]string{
	// v1
	"latest":       "1/latest",
	"crypto":       "1/crypto",
	"archive":      "1/archive",
	"sources":      "1/sources",
	"market":       "1/market",
	"count":        "1/count",
	"crypto_count": "1/crypto/count",
	"market_count": "1/market/count",
	// v2
	"news_v2":         "2/news",
	"market_v2":       "2/market",
	"crypto_v2":       "2/crypto",
	"sources_v2":      "2/sources",
	"count_news_v2":   "2/count/news",
	"count_market_v2": "2/count/market",
	"count_crypto_v2": "2/count/crypto",
	// The real-time service is v2 only.
	"websocket_register": "2/websocket/register",
	"websocket_fetch":    "2/websocket/fetch",
	"websocket_delete":   "2/websocket/delete",
}

// HTTP method per endpoint; anything absent is a GET.
var endpointMethods = map[string]string{
	"websocket_register": http.MethodPost,
	"websocket_delete":   http.MethodDelete,
}

// The websocket management endpoints answer with a success envelope that may
// carry no `results` field at all, so they are exempt from the results-present
// check `do` applies to the news endpoints.
var resultsOptional = setOf("websocket_register", "websocket_fetch", "websocket_delete")

// Error codes on a 429 that waiting cannot fix, so they are never retried.
// ApiLimitExceeded and ApiKeyLimitExceeded mean the API credits are exhausted
// rather than a transient rate limit — backoff cannot conjure more credits.
// RegistrationLimitExceeded means every real-time registration the plan allows
// is in use; only deleting one frees a slot.
//
// ApiLimitExceeded is the documented code (see the ErrorCode enum in
// https://newsdata.io/openapi.json); ApiKeyLimitExceeded is accepted too
// because the API has been observed to send it and the spec is not exhaustive.
var quotaExhaustedCodes = setOf(
	"ApiLimitExceeded", "ApiKeyLimitExceeded", "RegistrationLimitExceeded",
)

// Error code on a 429 → the error to return. None of them is worth retrying,
// but the remedy differs: ApiLimitExceeded is the account's whole credit pool,
// ApiKeyLimitExceeded is this one key's own cap, and RegistrationLimitExceeded
// is the real-time registration slots — it keeps the base error, since it is
// not about credits at all.
var quotaExhaustedErrors = map[string]func(*NewsdataRateLimitError) error{
	"ApiLimitExceeded": func(e *NewsdataRateLimitError) error {
		return &NewsdataAPILimitExceededError{NewsdataRateLimitError: e}
	},
	"ApiKeyLimitExceeded": func(e *NewsdataRateLimitError) error {
		return &NewsdataAPIKeyLimitExceededError{NewsdataRateLimitError: e}
	},
	"RegistrationLimitExceeded": func(e *NewsdataRateLimitError) error { return e },
}

// Endpoints that require both from_date and to_date.
var requiresDateRange = setOf("count", "crypto_count", "market_count")

// Type-classified parameter sets (lowercase API names).
var (
	// Includes the v2 spellings: "content" and "remove_duplicate".
	boolParams = setOf(
		"full_content", "image", "video", "removeduplicate",
		"content", "remove_duplicate",
	)
	intParams   = setOf("size")
	floatParams = setOf("sentiment_score")
)

// Mutually-exclusive parameter groups (lowercase API names).
var mutexGroups = [][]string{
	{"q", "qintitle", "qinmeta"},
	{"country", "excludecountry"},
	{"category", "excludecategory"},
	{"language", "excludelanguage"},
	{"domain", "domainurl", "excludedomain", "excludedomainurl"},
	// v2 names
	{"q", "q_in_title", "q_in_meta"},
	{"country", "exclude_country"},
	{"category", "exclude_category"},
	{"language", "exclude_language"},
	{"source_url", "exclude_source_url"},
}

// Per-endpoint accepted parameters (lowercase API names). Mirrors the server's
// FILTERS_MAPPING and the Python/PHP/Node clients.
var filters = map[string]map[string]bool{
	"latest": setOf(
		"q", "qintitle", "qinmeta", "country", "excludecountry", "category",
		"excludecategory", "language", "excludelanguage", "domain",
		"domainurl", "excludedomain", "excludedomainurl", "prioritydomain",
		"timeframe", "timezone", "size", "full_content", "image", "video",
		"page", "tag", "sentiment", "region", "excludefield",
		"removeduplicate", "id", "organization", "url", "sort", "creator",
		"datatype", "sentiment_score",
	),
	"archive": setOf(
		"q", "qintitle", "qinmeta", "country", "excludecountry", "category",
		"excludecategory", "language", "excludelanguage", "domain",
		"domainurl", "excludedomain", "excludedomainurl", "prioritydomain",
		"timezone", "size", "full_content", "image", "video", "page",
		"from_date", "to_date", "excludefield", "id", "url", "sort", "tag",
		"sentiment", "sentiment_score", "region", "organization", "creator",
		"datatype", "removeduplicate",
	),
	"crypto": setOf(
		"q", "qintitle", "qinmeta", "language", "excludelanguage", "domain",
		"domainurl", "excludedomain", "excludedomainurl", "prioritydomain",
		"timeframe", "timezone", "size", "full_content", "image", "video",
		"page", "tag", "sentiment", "coin", "excludefield", "from_date",
		"to_date", "removeduplicate", "id", "url", "sort",
	),
	"sources": setOf(
		"country", "category", "language", "prioritydomain", "domainurl",
	),
	"market": setOf(
		"q", "qintitle", "qinmeta", "from_date", "to_date", "country",
		"excludecountry", "domain", "domainurl", "excludedomain",
		"excludedomainurl", "language", "excludelanguage", "prioritydomain",
		"timezone", "timeframe", "size", "full_content", "image", "video",
		"page", "tag", "sentiment", "excludefield", "removeduplicate",
		"organization", "market_id", "id", "url", "sort", "creator",
		"datatype", "sentiment_score",
	),
	"count": setOf(
		"from_date", "to_date", "q", "qintitle", "qinmeta", "country",
		"excludecountry", "category", "excludecategory", "language",
		"excludelanguage", "domain", "domainurl", "excludedomain",
		"excludedomainurl", "full_content", "image", "video", "prioritydomain",
		"page", "size", "sort", "interval", "tag", "sentiment",
		"sentiment_score", "region", "organization", "creator", "datatype",
		"removeduplicate",
	),
	"crypto_count": setOf(
		"from_date", "to_date", "q", "qintitle", "qinmeta", "language",
		"excludelanguage", "coin", "domain", "domainurl", "excludedomain",
		"excludedomainurl", "full_content", "image", "video", "prioritydomain",
		"page", "sentiment", "size", "sort", "tag", "interval",
		"removeduplicate",
	),
	"market_count": setOf(
		"from_date", "to_date", "q", "qintitle", "qinmeta", "country",
		"excludecountry", "domain", "domainurl", "excludedomain",
		"excludedomainurl", "language", "excludelanguage", "full_content",
		"image", "video", "organization", "market_id", "prioritydomain",
		"page", "sentiment", "removeduplicate", "size", "sort", "tag",
		"interval", "creator", "datatype", "sentiment_score",
	),

	// ---- v2 ---------------------------------------------------------------
	// v2 renames the filter vocabulary to snake_case and folds
	// domain/domainurl/excludedomain/excludedomainurl into
	// source_url/exclude_source_url.
	"news_v2": setOf(
		"q", "q_in_title", "q_in_meta", "country", "exclude_country",
		"category", "exclude_category", "language", "exclude_language",
		"source_url", "exclude_source_url", "source_priority", "from_date",
		"to_date", "timeframe", "timezone", "size", "content", "image",
		"video", "page", "tag", "sentiment", "sentiment_score", "region",
		"organization", "creator", "datatype", "exclude_field",
		"remove_duplicate", "id", "url", "sort",
	),
	"market_v2": setOf(
		"q", "q_in_title", "q_in_meta", "country", "exclude_country",
		"language", "exclude_language", "source_url", "exclude_source_url",
		"source_priority", "from_date", "to_date", "timeframe", "timezone",
		"size", "content", "image", "video", "page", "tag", "sentiment",
		"sentiment_score", "organization", "market_id", "creator", "datatype",
		"exclude_field", "remove_duplicate", "id", "url", "sort",
	),
	"crypto_v2": setOf(
		"q", "q_in_title", "q_in_meta", "coin", "language", "exclude_language",
		"source_url", "exclude_source_url", "source_priority", "from_date",
		"to_date", "timeframe", "timezone", "size", "content", "image",
		"video", "page", "tag", "sentiment", "exclude_field",
		"remove_duplicate", "id", "url", "sort",
	),
	"sources_v2": setOf(
		"country", "category", "language", "source_priority", "source_url",
	),
	"count_news_v2": setOf(
		"q", "q_in_title", "q_in_meta", "country", "exclude_country",
		"category", "exclude_category", "language", "exclude_language",
		"source_url", "exclude_source_url", "source_priority", "from_date",
		"to_date", "timeframe", "interval", "size", "content", "image",
		"video", "page", "tag", "sentiment", "sentiment_score", "region",
		"organization", "creator", "datatype", "remove_duplicate", "sort",
	),
	"count_market_v2": setOf(
		"q", "q_in_title", "q_in_meta", "country", "exclude_country",
		"language", "exclude_language", "source_url", "exclude_source_url",
		"source_priority", "from_date", "to_date", "timeframe", "interval",
		"size", "content", "image", "video", "page", "tag", "sentiment",
		"sentiment_score", "organization", "market_id", "creator", "datatype",
		"remove_duplicate", "sort",
	),
	"count_crypto_v2": setOf(
		"q", "q_in_title", "q_in_meta", "coin", "language", "exclude_language",
		"source_url", "exclude_source_url", "source_priority", "from_date",
		"to_date", "timeframe", "interval", "size", "content", "image",
		"video", "page", "tag", "sentiment", "remove_duplicate", "sort",
	),
	// Real-time query registration. The real-time service is v2 only, so these
	// are the v2 filter names. No date or paging filters apply — a registered
	// query matches news as it is published. `news_type` is set by
	// WebSocketRegister, not by the caller.
	"websocket_register": setOf(
		"q", "q_in_title", "q_in_meta", "country", "exclude_country",
		"category", "exclude_category", "language", "exclude_language",
		"source_url", "exclude_source_url", "source_priority", "timezone",
		"content", "image", "video", "remove_duplicate", "tag", "sentiment",
		"sentiment_score", "region", "organization", "creator", "datatype",
		"exclude_field", "news_type",
	),
	"websocket_fetch":  setOf(),
	"websocket_delete": setOf("registration_id"),
}

func setOf(items ...string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s

}
