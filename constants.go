package newsdataapi

import (
	"net/http"
	"time"
)

// API host.
const baseURL = "https://newsdata.io/api/1/"

// Real-time WebSocket defaults (see WebSocket / NewWebSocket).
const (
	wsBaseURL = "wss://ws.newsdata.io/ws/event"
	// wsNewsType is the feed a registered query matches against.
	wsNewsType = "latest"
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
	"latest":             "latest",
	"crypto":             "crypto",
	"archive":            "archive",
	"sources":            "sources",
	"market":             "market",
	"count":              "count",
	"crypto_count":       "crypto/count",
	"market_count":       "market/count",
	"websocket_register": "websocket/register",
	"websocket_fetch":    "websocket/fetch",
	"websocket_delete":   "websocket/delete",
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

// Endpoints that require both from_date and to_date.
var requiresDateRange = setOf("count", "crypto_count", "market_count")

// Type-classified parameter sets (lowercase API names).
var (
	boolParams  = setOf("full_content", "image", "video", "removeduplicate")
	intParams   = setOf("size")
	floatParams = setOf("sentiment_score")
)

// Mutually-exclusive parameter groups (lowercase API names).
var mutexGroups = [][]string{
	{"q", "qintitle", "qinmeta"},
	{"country", "excludecountry"},
	{"category", "excludecategory"},
	{"language", "excludelanguage"},
	{"domain", "domainurl", "excludedomain"},
}

// Per-endpoint accepted parameters (lowercase API names). Mirrors the server's
// FILTERS_MAPPING and the Python/PHP/Node clients.
var filters = map[string]map[string]bool{
	"latest": setOf(
		"q", "qintitle", "qinmeta", "country", "excludecountry", "category",
		"excludecategory", "language", "excludelanguage", "domain", "domainurl",
		"excludedomain", "prioritydomain", "timeframe", "timezone", "size",
		"full_content", "image", "video", "page", "tag", "sentiment", "region",
		"excludefield", "removeduplicate", "id", "organization", "url", "sort",
		"creator", "datatype", "sentiment_score",
	),
	"archive": setOf(
		"q", "qintitle", "qinmeta", "country", "excludecountry", "category",
		"excludecategory", "language", "excludelanguage", "domain", "domainurl",
		"excludedomain", "prioritydomain", "timezone", "size", "full_content",
		"image", "video", "page", "from_date", "to_date", "excludefield", "id",
		"url", "sort", "tag", "sentiment", "sentiment_score", "region",
		"organization", "creator", "datatype", "removeduplicate",
	),
	"crypto": setOf(
		"q", "qintitle", "qinmeta", "language", "excludelanguage", "domain",
		"domainurl", "excludedomain", "prioritydomain", "timeframe", "timezone",
		"size", "full_content", "image", "video", "page", "tag", "sentiment",
		"coin", "excludefield", "from_date", "to_date", "removeduplicate", "id",
		"url", "sort",
	),
	"sources": setOf(
		"country", "category", "language", "prioritydomain", "domainurl",
	),
	"market": setOf(
		"q", "qintitle", "qinmeta", "from_date", "to_date", "country",
		"excludecountry", "domain", "domainurl", "excludedomain", "language",
		"excludelanguage", "prioritydomain", "timezone", "timeframe", "size",
		"full_content", "image", "video", "page", "tag", "sentiment",
		"excludefield", "removeduplicate", "organization", "market_id", "id", "url",
		"sort", "creator", "datatype", "sentiment_score",
	),
	"count": setOf(
		"from_date", "to_date", "q", "qintitle", "qinmeta", "country",
		"excludecountry", "category", "excludecategory", "language",
		"excludelanguage", "domain", "domainurl", "excludedomain", "full_content",
		"image", "video", "prioritydomain", "page", "size", "sort", "interval",
		"tag", "sentiment", "sentiment_score", "region", "organization", "creator",
		"datatype", "removeduplicate",
	),
	"crypto_count": setOf(
		"from_date", "to_date", "q", "qintitle", "qinmeta", "language",
		"excludelanguage", "coin", "domain", "domainurl", "excludedomain",
		"full_content", "image", "video", "prioritydomain", "page", "sentiment",
		"size", "sort", "tag", "interval", "removeduplicate",
	),
	"market_count": setOf(
		"from_date", "to_date", "q", "qintitle", "qinmeta", "country",
		"excludecountry", "domain", "domainurl", "excludedomain", "language",
		"excludelanguage", "full_content", "image", "video", "organization",
		"market_id", "prioritydomain", "page", "sentiment", "removeduplicate", "size",
		"sort", "tag", "interval", "creator", "datatype", "sentiment_score",
	),
	// Real-time query registration. No date/paging filters — a registered
	// query matches news as it is published. `news_type` is set by
	// WebSocketRegister, not by the caller.
	"websocket_register": setOf(
		"q", "qintitle", "qinmeta", "country", "excludecountry", "category",
		"excludecategory", "language", "excludelanguage", "domain", "domainurl",
		"excludedomain", "prioritydomain", "timezone", "full_content", "image",
		"video", "removeduplicate", "tag", "sentiment", "sentiment_score",
		"region", "organization", "creator", "datatype", "excludefield",
		"news_type",
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
