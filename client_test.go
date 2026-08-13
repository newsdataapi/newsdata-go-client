package newsdataapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// successBody builds a minimal "status:success" envelope wrapping `results` (a
// JSON array fragment).
func successBody(resultsJSON string) string {
	return `{"status":"success","results":` + resultsJSON + `}`
}

func TestSuccessfulRequest(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		_, _ = w.Write([]byte(successBody(`[{"title":"a","article_id":"1"}]`)))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Latest(context.Background(), Params{"q": "x"})
	if err != nil {
		t.Fatal(err)
	}
	articles, err := resp.Articles()
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].Title != "a" {
		t.Errorf("unexpected response: %+v", articles)
	}
	if !strings.Contains(capturedURL, "apikey=key") {
		t.Errorf("apikey missing in URL: %s", capturedURL)
	}
}

// Market results carry both `symbol` and `market_id`; the two are separate
// response fields and both decode onto Article.
func TestMarketArticleDecodesSymbolAndMarketID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(successBody(
			`[{"article_id":"m1","symbol":["AAPL","MSFT"],"market_id":["NASDAQ:AAPL","NASDAQ:MSFT"]}]`)))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Market(context.Background(), Params{"market_id": "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	articles, err := resp.Articles()
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if got := strings.Join(articles[0].Symbol, ","); got != "AAPL,MSFT" {
		t.Errorf("Symbol = %q, want \"AAPL,MSFT\"", got)
	}
	if got := strings.Join(articles[0].MarketID, ","); got != "NASDAQ:AAPL,NASDAQ:MSFT" {
		t.Errorf("MarketID = %q, want \"NASDAQ:AAPL,NASDAQ:MSFT\"", got)
	}
}

func TestAuthError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"status":"error","results":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL))
	_, err := c.Latest(context.Background(), Params{"q": "x"})

	var authErr *NewsdataAuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected NewsdataAuthError, got %T: %v", err, err)
	}
	if authErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", authErr.StatusCode)
	}

	// Should also match the parent APIError via errors.As.
	var apiErr *NewsdataAPIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected to also match NewsdataAPIError")
	}
}

func TestRateLimitRetryThenError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		retryAfter := "0"
		if n == 2 {
			retryAfter = "7"
		}
		w.Header().Set("Retry-After", retryAfter)
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"status":"error"}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key",
		WithBaseURL(srv.URL),
		WithMaxRetries(2),
		WithRetryBackoff(time.Millisecond),
		WithRetryBackoffMax(time.Millisecond),
	)
	_, err := c.Latest(context.Background(), Params{"q": "x"})

	var rl *NewsdataRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected NewsdataRateLimitError, got %T: %v", err, err)
	}
	if rl.RetryAfter != 7 {
		t.Errorf("RetryAfter = %d, want 7", rl.RetryAfter)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestServerErrorRetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"status":"error"}`))
			return
		}
		_, _ = w.Write([]byte(successBody(`[{"title":"recovered","article_id":"1"}]`)))
	}))
	defer srv.Close()

	c, _ := NewClient("key",
		WithBaseURL(srv.URL),
		WithMaxRetries(3),
		WithRetryBackoff(time.Millisecond),
	)
	resp, err := c.Latest(context.Background(), Params{"q": "x"})
	if err != nil {
		t.Fatal(err)
	}
	arts, _ := resp.Articles()
	if len(arts) != 1 || arts[0].Title != "recovered" {
		t.Errorf("unexpected resp: %+v", arts)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestScrollAllMergesPages(t *testing.T) {
	pages := [][]byte{
		[]byte(`{"status":"success","totalResults":3,"nextPage":"p2","results":[{"title":"a","article_id":"1"},{"title":"b","article_id":"2"}]}`),
		[]byte(`{"status":"success","totalResults":3,"results":[{"title":"c","article_id":"3"}]}`),
	}
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		_, _ = w.Write(pages[n-1])
	}))
	defer srv.Close()

	c, _ := NewClient("key",
		WithBaseURL(srv.URL),
		WithPaginationDelay(0),
	)
	resp, err := c.ScrollAll(context.Background(), EndpointLatest, Params{"q": "x"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	arts, _ := resp.Articles()
	if len(arts) != 3 {
		t.Errorf("got %d articles, want 3", len(arts))
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestScrollAllHonorsMaxResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","nextPage":"p2","results":[{"title":"a","article_id":"1"},{"title":"b","article_id":"2"}]}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL), WithPaginationDelay(0))
	resp, err := c.ScrollAll(context.Background(), EndpointLatest, Params{"q": "x"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	arts, _ := resp.Articles()
	if len(arts) != 1 {
		t.Errorf("got %d articles, want 1", len(arts))
	}
}

func TestPaginateYieldsEachPage(t *testing.T) {
	pages := [][]byte{
		[]byte(`{"status":"success","nextPage":"p2","results":[{"title":"a","article_id":"1"}]}`),
		[]byte(`{"status":"success","results":[{"title":"b","article_id":"2"}]}`),
	}
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		_, _ = w.Write(pages[n-1])
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL), WithPaginationDelay(0))

	var seen []string
	err := c.Paginate(context.Background(), EndpointLatest, Params{"q": "x"},
		func(r *Response, e error) bool {
			if e != nil {
				t.Errorf("yield error: %v", e)
				return false
			}
			arts, _ := r.Articles()
			for _, a := range arts {
				seen = append(seen, a.Title)
			}
			return true
		})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "a,b" {
		t.Errorf("seen = %v", seen)
	}
}

func TestPaginateStopsWhenYieldReturnsFalse(t *testing.T) {
	pages := [][]byte{
		[]byte(`{"status":"success","nextPage":"p2","results":[{"title":"a","article_id":"1"}]}`),
		[]byte(`{"status":"success","nextPage":"p3","results":[{"title":"b","article_id":"2"}]}`),
		[]byte(`{"status":"success","results":[{"title":"c","article_id":"3"}]}`),
	}
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		_, _ = w.Write(pages[n-1])
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL), WithPaginationDelay(0))

	count := 0
	_ = c.Paginate(context.Background(), EndpointLatest, Params{"q": "x"},
		func(r *Response, e error) bool {
			count++
			return count < 2
		})
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("server calls = %d, want 2", calls)
	}
}

func TestEmptyAPIKeyRejected(t *testing.T) {
	_, err := NewClient("")
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected NewsdataValidationError, got %v", err)
	}
}

func TestRedactAPIKey(t *testing.T) {
	in := "https://newsdata.io/api/1/latest?apikey=SECRET&q=foo"
	want := "https://newsdata.io/api/1/latest?apikey=REDACTED&q=foo"
	if got := redactAPIKey(in); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestContextCancellationStopsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"status":"error"}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key",
		WithBaseURL(srv.URL),
		WithMaxRetries(10),
		WithRetryBackoff(time.Second),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	_, err := c.Latest(ctx, Params{"q": "x"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// Should be a network error wrapping context.Canceled
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in chain, got %v", err)
	}
}
