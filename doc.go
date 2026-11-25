// Package newsdataapi is the official Go client for the Newsdata.io REST API.
//
// It wraps every endpoint of both API versions with client-side parameter
// validation, retries with exponential backoff, scroll/paginate helpers, and a
// typed error hierarchy:
//
//   - v1: Latest, Archive, Sources, Crypto, Market, Count, CryptoCount,
//     MarketCount;
//   - v2: NewsV2, MarketV2, CryptoV2, SourcesV2, CountNewsV2, CountMarketV2,
//     CountCryptoV2, plus the real-time service (WebSocketRegister,
//     WebSocketFetch, WebSocketDelete and WebSocket.Stream), which is v2 only.
//
// v2 takes the snake_case filter vocabulary (q_in_title, exclude_country,
// source_url, source_priority, content, remove_duplicate, ...) and answers
// with total_results / next_page, which Response decodes into TotalResults /
// NextPage so callers need not branch on the version.
//
// Quickstart:
//
//	client, err := newsdataapi.NewClient("YOUR_API_KEY")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, err := client.Latest(context.Background(), newsdataapi.LatestParams{
//	    Q:        "bitcoin",
//	    Country:  []string{"us", "gb"},
//	    Language: []string{"en"},
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	articles, _ := resp.Articles()
//	for _, a := range articles {
//	    fmt.Println(a.Title, "-", a.Link)
//	}
//
// All methods accept a context.Context for cancellation and deadlines. Errors
// are typed (NewsdataValidationError, NewsdataAuthError, NewsdataRateLimitError,
// NewsdataServerError, NewsdataAPIError, NewsdataNetworkError) and can be
// inspected with errors.As.
package newsdataapi
