<div align="center">

![Newsdata.io logo](https://raw.githubusercontent.com/newsdataapi/newsdata-go-client/main/newsdata-logo.png)

# Newsdata.io Go Client

[![Go Reference](https://pkg.go.dev/badge/github.com/newsdataapi/newsdata-go-client.svg)](https://pkg.go.dev/github.com/newsdataapi/newsdata-go-client)
[![CI](https://img.shields.io/github/actions/workflow/status/newsdataapi/newsdata-go-client/ci.yml?branch=main&logo=github&label=CI)](https://github.com/newsdataapi/newsdata-go-client/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-%3E%3D1.18-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.1-85EA2D)](https://newsdata.io/openapi.json)

</div>

Official Go client for the [Newsdata.io](https://newsdata.io) News API. Wraps
every endpoint (`latest`, `archive`, `sources`, `crypto`, `market`, `count`,
`crypto/count`, `market/count`) with client-side parameter validation,
automatic retries with exponential backoff, scroll/paginate helpers, and a
typed error hierarchy. Idiomatic Go: `context.Context`-aware, no external
runtime dependencies, safe for concurrent use.

## Installation

```bash
go get github.com/newsdataapi/newsdata-go-client@latest
```

Go modules pull the package straight from GitHub — no separate registry to
configure.

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/newsdataapi/newsdata-go-client"
)

func main() {
    client, err := newsdataapi.NewClient(os.Getenv("NEWSDATA_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Latest(context.Background(), newsdataapi.Params{
        "q":        "bitcoin",
        "country":  []string{"us", "gb"},
        "language": "en",
    })
    if err != nil {
        log.Fatal(err)
    }

    articles, _ := resp.Articles()
    for _, a := range articles {
        fmt.Println(a.Title, "-", a.Link)
    }
}
```

## Endpoints

The API has two versions. v2 is the current one; the v1 methods keep working
unchanged.

### v2

| Method | Endpoint | Notes |
|--------|----------|-------|
| `client.NewsV2(ctx, params)` | `/2/news` | Latest news, or historical with `from_date` / `to_date` |
| `client.MarketV2(ctx, params)` | `/2/market` | Market / financial news |
| `client.CryptoV2(ctx, params)` | `/2/crypto` | Cryptocurrency news |
| `client.SourcesV2(ctx, params)` | `/2/sources` | Available sources (single page) |
| `client.CountNewsV2(ctx, params)` | `/2/count/news` | Aggregate news counts |
| `client.CountMarketV2(ctx, params)` | `/2/count/market` | Aggregate market counts |
| `client.CountCryptoV2(ctx, params)` | `/2/count/crypto` | Aggregate crypto counts |
| `client.WebSocketRegister(ctx, params)` | `/2/websocket/register` | Register a real-time query |
| `client.WebSocketFetch(ctx)` | `/2/websocket/fetch` | List registered queries |
| `client.WebSocketDelete(ctx, id)` | `/2/websocket/delete` | Delete a registered query |

v2 renames the filter vocabulary to snake_case and folds the four domain
filters into two:

| v1 | v2 |
|----|----|
| `qInTitle`, `qInMeta` | `q_in_title`, `q_in_meta` |
| `excludecountry`, `excludecategory`, `excludelanguage` | `exclude_country`, `exclude_category`, `exclude_language` |
| `domain`, `domainurl`, `excludedomain`, `excludedomainurl` | `source_url`, `exclude_source_url` |
| `prioritydomain` | `source_priority` |
| `full_content` | `content` |
| `removeduplicate`, `excludefield` | `remove_duplicate`, `exclude_field` |

`coin` and `market_id` keep their names.

```go
resp, err := client.NewsV2(ctx, newsdataapi.Params{
    "q_in_title": "tesla",
    "source_url": "bbc.com",
    "content":    true,
})
```

The v2 envelope spells its total and cursor `total_results` / `next_page`
where v1 spells them `totalResults` / `nextPage`. `Response` decodes either
into `TotalResults` / `NextPage`, so `ScrollAll`, `Paginate` and your own code
read the same fields whichever version answered.

There is no `/2/archive` — `NewsV2` covers historical news through
`from_date` / `to_date`.

#### v2 articles

v2 renamed and regrouped the article fields, so the v2 endpoints and the
real-time stream decode with `ArticlesV2()` and `ArticleV2`, while the v1
endpoints keep `Articles()` and `Article`:

```go
resp, _ := client.NewsV2(ctx, newsdataapi.Params{"q": "bitcoin"})

articles, err := resp.ArticlesV2()
if err != nil {
    log.Fatal(err)
}
for _, a := range articles {
    fmt.Println(a.Title, a.URL, a.PublishedAt)
    fmt.Println(a.Source.Name, a.Source.Priority.Int())
    fmt.Println(a.Media.ImageURL)
    fmt.Println(a.AI.Summary, a.AI.Sentiment.Label, a.AI.Tag)
}
```

| v1 (`Article`) | v2 (`ArticleV2`) |
|----------------|------------------|
| `ArticleID` | `ID` |
| `Link` | `URL` |
| `PubDate` | `PublishedAt` |
| `Description` | `MetaDescription` |
| `Keywords` | `Keyword` |
| `Symbol` | `Ticker` |
| `SourceName`, `SourceURL`, `SourcePriority`, `SourceIcon` | `Source.Name`, `Source.URL`, `Source.Priority`, `Source.IconURL` |
| `ImageURL`, `VideoURL` | `Media.ImageURL`, `Media.VideoURL` |
| `Sentiment`, `SentimentStats` | `AI.Sentiment.Label`, `AI.Sentiment.Score` |
| `AITag`, `AIRegion`, `AIOrg` | `AI.Tag`, `AI.Region`, `AI.Organization` |
| — | `AI.Summary`, `FetchedAt`, `Duplicate`, `Coin`, `MarketID` |

A field your plan does not include arrives as the API's notice string in place
of the data (`"ONLY AVAILABLE IN PAID PLANS"`). Decoding tolerates it — a list
field holds the single notice — and `newsdataapi.IsPlanNotice(value)` tells it
apart from real data.

### v1

| Method | Endpoint | Notes |
|--------|----------|-------|
| `client.Latest(ctx, params)` | `/1/latest` | Real-time news |
| `client.Archive(ctx, params)` | `/1/archive` | Historical news |
| `client.Sources(ctx, params)` | `/1/sources` | Available sources (single page) |
| `client.Crypto(ctx, params)` | `/1/crypto` | Cryptocurrency news |
| `client.Market(ctx, params)` | `/1/market` | Market / financial news |
| `client.Count(ctx, params)` | `/1/count` | Aggregate counts (requires `from_date`, `to_date`) |
| `client.CryptoCount(ctx, params)` | `/1/crypto/count` | Aggregate crypto counts |
| `client.MarketCount(ctx, params)` | `/1/market/count` | Aggregate market counts |

The real-time service is v2 only — there are no v1 WebSocket methods.

Every value in `Params` may be a `string`, a `[]string` (sent comma-joined),
a `bool`, an `int`, a `float64`, or `*bool` / `*float64` if you need to
distinguish "unset" from a zero value. Parameter names are case-insensitive —
`qInTitle` and `qintitle` are equivalent.

## Pagination

Two helpers; both take an endpoint constant (`EndpointLatest`, `EndpointArchive`, …):

```go
// 1) ScrollAll: follow nextPage cursors, return one merged Response.
all, err := client.ScrollAll(ctx, newsdataapi.EndpointLatest,
    newsdataapi.Params{"q": "news"}, 200)

articles, _ := all.Articles() // all 200 in one slice

// 2) Paginate: callback per page. Return false from yield to stop early.
err = client.Paginate(ctx, newsdataapi.EndpointLatest,
    newsdataapi.Params{"q": "news"},
    func(page *newsdataapi.Response, err error) bool {
        if err != nil {
            return false
        }
        arts, _ := page.Articles()
        process(arts)
        return true
    })
```

## Raw query

```go
resp, err := client.Latest(ctx, newsdataapi.Params{
    "rawQuery": "q=bitcoin&country=us&language=en",
})
```

`rawQuery` is mutually exclusive with all other parameters and is validated
against the endpoint's allowed keys before the request is sent.

## Client-side validation

Before any request leaves the process, parameters are validated and a typed
`*NewsdataValidationError` is returned (no API quota spent) when:

- a parameter is not accepted by that endpoint;
- mutually-exclusive parameters are set together — `q`/`qInTitle`/`qInMeta`,
  `country`/`excludecountry`, `category`/`excludecategory`,
  `language`/`excludelanguage`, `domain`/`domainurl`/`excludedomain`;
- `size` is outside 1–50;
- `sentiment_score` is set without `sentiment`;
- a count endpoint is missing `from_date` or `to_date`.

Booleans for `full_content`, `image`, `video`, and `removeduplicate` are
coerced to `"1"`/`"0"`.

## Real-time news (WebSocket)

The real-time service is part of API v2. Register a query first — the returned
`registration_id` identifies it from then on:

```go
resp, err := client.WebSocketRegister(ctx, newsdataapi.Params{
    "q":        "bitcoin",
    "language": "en",
})
agg, _ := resp.Aggregate()
registrationID := agg["registration_id"].(string)
```

`WebSocketRegister` takes the same filter names as `NewsV2` (`q`, `q_in_title`,
`country`, `exclude_country`, `source_url`, `content`, `exclude_field`,
`timezone`, …) — the date, paging and sorting filters, `id` and `url` don't
apply to a standing query. Registering an identical query twice answers HTTP
409 with the existing id in the error's `ResponseBody`.

`WebSocketFetch` lists every registered query: `Results` is a JSON array with
one entry per query, carrying its `registration_id`, the filters it was
registered with (`query`), how many devices are streaming it now
(`active_clients`) and when one last connected (`last_used`, UTC) — unmarshal
it into your own type. `WebSocketDelete` removes one. Once every registration
the plan allows is in use, registering another returns a 429
`NewsdataRateLimitError` carrying the code `RegistrationLimitExceeded`; it is
not retried, since only deleting a registration frees a slot.

Then stream. Each response carries `status`, `total_results` and `results` —
decoded into `Status`, `TotalResults` and `Results` — and its articles use the
v2 layout, so decode them with `ArticlesV2()`:

```go
ws := newsdataapi.NewWebSocket(client)

err = ws.Stream(ctx, registrationID, func(resp *newsdataapi.Response) error {
    articles, err := resp.ArticlesV2()
    if err != nil {
        return err
    }
    for _, a := range articles {
        fmt.Println(a.Title, "-", a.URL, a.Source.Name)
    }
    return nil
})
```

`Stream` blocks until the context is cancelled or the handler returns an error.
Return `newsdataapi.ErrStopStream` to stop cleanly — `Stream` then returns nil:

```go
err = ws.Stream(ctx, registrationID, func(resp *newsdataapi.Response) error {
    fmt.Println(resp.TotalResults)
    return newsdataapi.ErrStopStream
})
```

Transient drops (network errors, server restarts, abnormal closes) are
reconnected automatically with a capped exponential backoff. Pass
`WithWSReconnect(false)` to stop on the first disconnect instead. A permanent
rejection — bad API key or unknown
`registration_id`, exhausted API credits, or too many simultaneous devices — returns
`*NewsdataWebSocketAuthError` and is **not** retried.

The server always accepts the handshake and then closes with code **1008** when
the connection is refused, carrying one of three reasons: `invalid credentials
or registration not found`, `api limit reached`, or `device limit reached` (more
than 5 devices on one `registration_id`). Every other close code — including
`1013` (`send timeout`, meaning the client read too slowly) — is transient and
reconnects.

**Each delivered article consumes 1 API credit per connected device.**

Catch it like any other client error:

```go
var authErr *newsdataapi.NewsdataWebSocketAuthError
if errors.As(err, &authErr) {
    log.Fatalf("rejected: %v", err)
}
```

All connection options are functional:

```go
ws := newsdataapi.NewWebSocket(client,
    newsdataapi.WithWSBaseURL("wss://ws.newsdata.io/ws/event"), // staging / self-hosted
    newsdataapi.WithWSReconnect(true),                          // default true
    newsdataapi.WithWSReconnectDelay(time.Second, 30*time.Second), // first delay, cap
    newsdataapi.WithWSHandshakeTimeout(10*time.Second),         // 0 disables
    newsdataapi.WithWSKeepalive(20*time.Second, 20*time.Second),// ping interval, pong wait
    newsdataapi.WithWSHeaders(http.Header{"X-Trace": {"abc"}}), // extra handshake headers
    newsdataapi.WithWSProxy(http.ProxyFromEnvironment),         // proxy
)
```

Runnable example: [`examples/websocket`](examples/websocket).

## Error handling

All SDK errors satisfy the typed hierarchy and play nicely with `errors.As`
and `errors.Is`:

```go
import "errors"

var (
    ve *newsdataapi.NewsdataValidationError
    ae *newsdataapi.NewsdataAuthError
    rl *newsdataapi.NewsdataRateLimitError
    se *newsdataapi.NewsdataServerError
    ne *newsdataapi.NewsdataNetworkError
    api *newsdataapi.NewsdataAPIError
)

switch {
case errors.As(err, &ve): /* invalid parameter: ve.Param, ve.Message */
case errors.As(err, &ae): /* 401 / 403: ae.StatusCode */
case errors.As(err, &rl): /* 429: rl.RetryAfter (seconds) */
case errors.As(err, &se): /* 5xx */
case errors.As(err, &ne): /* network/timeout/cancellation: ne.Err */
case errors.As(err, &api): /* other API errors: api.StatusCode, api.Message */
}
```

Hierarchy:

```
NewsdataError                       (catch-all base)
NewsdataValidationError             (.Param, .Message)
NewsdataAPIError                    (.StatusCode, .Message, .ResponseBody)
├── NewsdataAuthError               (401 / 403)
├── NewsdataRateLimitError          (429; .RetryAfter)
│   ├── NewsdataAPILimitExceededError     (account credits gone)
│   └── NewsdataAPIKeyLimitExceededError  (this key's cap gone)
│   (RegistrationLimitExceeded stays on NewsdataRateLimitError —
│    real-time registration slots, not credits)
└── NewsdataServerError             (5xx)
NewsdataNetworkError                (.Err)
NewsdataWebSocketError              (.Message, .Err — real-time stream)
└── NewsdataWebSocketAuthError      (policy-violation close 1008)
```

## Configuration

```go
client, err := newsdataapi.NewClient(apiKey,
    newsdataapi.WithTimeout(30 * time.Second),         // per-request
    newsdataapi.WithMaxRetries(5),                     // 1 = no retry
    newsdataapi.WithRetryBackoff(2 * time.Second),     // base, exponential
    newsdataapi.WithRetryBackoffMax(60 * time.Second), // cap on a single sleep
    newsdataapi.WithPaginationDelay(time.Second),
    newsdataapi.WithBaseURL("https://staging.example/api"), // root, no version
    newsdataapi.WithHTTPClient(myCustomClient),        // proxies / mTLS
    newsdataapi.WithIncludeHeaders(true),              // attach Response.ResponseHeaders
    newsdataapi.WithLogger(myLogger),                  // any Info(msg)/Warn(msg) value
)
```

`WithBaseURL` takes the API root **without** a version segment; the default is
`https://newsdata.io/api` and the client adds `/1/…` or `/2/…` itself. Before
v0.3.0 it included the version, so drop the trailing `/1/` if you override it.

Retries cover network errors, HTTP 429, and 5xx responses. 429 honours the
`Retry-After` header (integer seconds or HTTP-date); otherwise the wait is
exponential. Auth and other 4xx errors are never retried. A 429 that waiting
cannot fix is never retried: `NewsdataAPILimitExceededError` means the
account's whole credit pool is exhausted, `NewsdataAPIKeyLimitExceededError`
means this one key hit its own cap, and both unwrap to
`NewsdataRateLimitError`, so an existing `errors.As(err, &rl)` keeps working.
`RegistrationLimitExceeded` is also never retried, but keeps the plain
`NewsdataRateLimitError` — it is about real-time registration slots, not
credits, and is fixed by deleting a registration. **`context.Context`
cancellation interrupts retries immediately** — passing a cancelled or
deadlined context returns a `NewsdataNetworkError` wrapping `context.Canceled`
or `context.DeadlineExceeded`.

## Concurrency

`Client` is safe for concurrent use by multiple goroutines.

## Development

```bash
go test -race ./...                # full unit suite (no API key required)
go vet ./...
go build ./examples/...
```

The test suite uses `net/http/httptest` to mock the API end-to-end — no
network access required. 30 tests cover the validator, request loop, retry
behaviour, scroll + paginate, error mapping, and API-key redaction.

## Related libraries

Official Newsdata.io clients across languages and runtimes:

- **Python** — [newsdataapi/python-client](https://github.com/newsdataapi/python-client) ([PyPI](https://pypi.org/project/newsdataapi/))
- **Node.js** — [newsdataapi/newsdata-nodejs-client](https://github.com/newsdataapi/newsdata-nodejs-client) ([npm](https://www.npmjs.com/package/newsdata-nodejs-client))
- **React (hooks)** — [newsdataapi/newsdata-reactjs-client](https://github.com/newsdataapi/newsdata-reactjs-client) ([npm](https://www.npmjs.com/package/newsdataapi))
- **PHP** — [newsdataapi/php-client](https://github.com/newsdataapi/php-client) ([Packagist](https://packagist.org/packages/newsdataio/newsdataapi))
- **Java** — [newsdataapi/newsdata-java-sdk](https://github.com/newsdataapi/newsdata-java-sdk) ([Maven Central](https://central.sonatype.com/artifact/io.newsdata/newsdataapi))
- **.NET** — [newsdataapi/newsdata-dotnet-sdk](https://github.com/newsdataapi/newsdata-dotnet-sdk) ([NuGet](https://www.nuget.org/packages/Newsdata.Api/))
- **Dart / Flutter** — [newsdataapi/newsdata-flutter-client](https://github.com/newsdataapi/newsdata-flutter-client) ([pub.dev](https://pub.dev/packages/newsdataapi))
- **MCP Server (AI assistants)** — [newsdataapi/newsdata.io-mcp](https://github.com/newsdataapi/newsdata.io-mcp) ([PyPI](https://pypi.org/project/newsdata-mcp/))

Also see [free news datasets](https://github.com/newsdataapi/newsdata.io-free-datasets) for ML / NLP work.

## License

[MIT](./LICENSE)
