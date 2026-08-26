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
| `client.WebSocketRegister(ctx, params)` | `/1/websocket/register` | Register a real-time query |
| `client.WebSocketFetch(ctx)` | `/1/websocket/fetch` | List registered queries |
| `client.WebSocketDelete(ctx, id)` | `/1/websocket/delete` | Delete a registered query |

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

Register a query first — the returned `registration_id` identifies it from then on:

```go
resp, err := client.WebSocketRegister(ctx, newsdataapi.Params{
    "q":        "bitcoin",
    "language": "en",
})
agg, _ := resp.Aggregate()
registrationID := agg["registration_id"].(string)
```

`WebSocketRegister` takes the familiar filter names (`q`, `country`, `language`,
`domain`, …) — no date or paging filters, since a registered query matches news
as it is published. Registering an identical query twice answers HTTP 409 with
the existing id in the error's `ResponseBody`. `WebSocketFetch` lists every
registered query and `WebSocketDelete` removes one.

Then stream. Each response has the familiar `status` / `totalResults` /
`results` shape:

```go
ws := newsdataapi.NewWebSocket(client)

err = ws.Stream(ctx, registrationID, func(resp *newsdataapi.Response) error {
    articles, err := resp.Articles()
    if err != nil {
        return err
    }
    for _, a := range articles {
        fmt.Println(a.Title, "-", a.Link)
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
    newsdataapi.WithBaseURL("https://staging.example/api/1/"),
    newsdataapi.WithHTTPClient(myCustomClient),        // proxies / mTLS
    newsdataapi.WithIncludeHeaders(true),              // attach Response.ResponseHeaders
    newsdataapi.WithLogger(myLogger),                  // any Info(msg)/Warn(msg) value
)
```

Retries cover network errors, HTTP 429, and 5xx responses. 429 honours the
`Retry-After` header (integer seconds or HTTP-date); otherwise the wait is
exponential. Auth and other 4xx errors are never retried. **`context.Context`
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
