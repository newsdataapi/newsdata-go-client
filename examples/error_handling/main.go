// Typed error handling with errors.As — same pattern as the Python / Node
// clients but using Go's idiomatic error chain.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/newsdataapi/newsdata-go-client"
)

func main() {
	client, err := newsdataapi.NewClient(os.Getenv("NEWSDATA_API_KEY"),
		newsdataapi.WithTimeout(15*time.Second),
		newsdataapi.WithMaxRetries(3),
		newsdataapi.WithRetryBackoff(2*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Count endpoints require from_date and to_date.
	dates := newsdataapi.Params{
		"from_date": "2024-01-01",
		"to_date":   "2024-01-31",
		"interval":  "day",
	}

	resp, err := client.Count(context.Background(), withMerge(dates, newsdataapi.Params{"q": "election"}))
	if err == nil {
		fmt.Println("count results:", string(resp.Results))
		return
	}

	// Type-switch on the typed error hierarchy.
	var (
		ve *newsdataapi.NewsdataValidationError
		ae *newsdataapi.NewsdataAuthError
		rl *newsdataapi.NewsdataRateLimitError
		se *newsdataapi.NewsdataServerError
		ne *newsdataapi.NewsdataNetworkError
		ge *newsdataapi.NewsdataAPIError
	)
	switch {
	case errors.As(err, &ve):
		fmt.Println("invalid param", ve.Param+":", ve.Message)
	case errors.As(err, &ae):
		fmt.Println("auth failed (HTTP", ae.StatusCode, ")")
	case errors.As(err, &rl):
		fmt.Println("rate limited; retry after", rl.RetryAfter, "seconds")
	case errors.As(err, &se):
		fmt.Println("server error", se.StatusCode)
	case errors.As(err, &ne):
		fmt.Println("network error:", ne.Err)
	case errors.As(err, &ge):
		fmt.Printf("API error %d: %s\n", ge.StatusCode, ge.Message)
	default:
		fmt.Println("unexpected:", err)
	}
}

func withMerge(a, b newsdataapi.Params) newsdataapi.Params {
	out := make(newsdataapi.Params, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
