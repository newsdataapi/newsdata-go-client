// Package newsdataapi is the official Go client for the Newsdata.io REST API.
//
// It wraps every endpoint (latest, archive, sources, crypto, market, count,
// crypto/count, market/count) with client-side parameter validation, retries
// with exponential backoff, scroll/paginate helpers, and a typed error
// hierarchy.
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
