package newsdataapi

import "time"

// API host.
const baseURL = "https://newsdata.io/api/1/"

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
	"latest":       "latest",
	"crypto":       "crypto",
	"archive":      "archive",
	"sources":      "sources",
	"market":       "market",
	"count":        "count",
	"crypto_count": "crypto/count",
	"market_count": "market/count",
}

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
}

func setOf(items ...string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}
