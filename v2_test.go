package newsdataapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// v2EndpointCalls maps each v2 client method to the path it must request.
func v2EndpointCalls(c *Client) []struct {
	name string
	path string
	call func(context.Context, Params) (*Response, error)
} {
	return []struct {
		name string
		path string
		call func(context.Context, Params) (*Response, error)
	}{
		{"NewsV2", "/2/news", c.NewsV2},
		{"MarketV2", "/2/market", c.MarketV2},
		{"CryptoV2", "/2/crypto", c.CryptoV2},
		{"SourcesV2", "/2/sources", c.SourcesV2},
		{"CountNewsV2", "/2/count/news", c.CountNewsV2},
		{"CountMarketV2", "/2/count/market", c.CountMarketV2},
		{"CountCryptoV2", "/2/count/crypto", c.CountCryptoV2},
	}
}

func TestV2EndpointPaths(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte(successBody(`[]`)))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range v2EndpointCalls(c) {
		// sources takes no `q`, so send a filter every v2 endpoint accepts.
		if _, err := tc.call(context.Background(), Params{"language": "en"}); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if capturedPath != tc.path {
			t.Errorf("%s requested %q, want %q", tc.name, capturedPath, tc.path)
		}
	}
}

func TestV1EndpointPathsStayVersioned(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte(successBody(`[]`)))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Latest(context.Background(), Params{"q": "x"}); err != nil {
		t.Fatal(err)
	}
	if capturedPath != "/1/latest" {
		t.Errorf("Latest requested %q, want /1/latest", capturedPath)
	}
}

func TestV2AcceptsSnakeCaseVocabulary(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(successBody(`[]`)))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.NewsV2(context.Background(), Params{
		"q_in_title":       "tesla",
		"exclude_country":  "cn",
		"source_url":       "bbc.com",
		"source_priority":  "top",
		"content":          true,
		"remove_duplicate": true,
		"exclude_field":    "description",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"q_in_title=tesla", "exclude_country=cn", "source_url=bbc.com",
		"source_priority=top", "content=1", "remove_duplicate=1",
		"exclude_field=description",
	} {
		if !strings.Contains(capturedQuery, want) {
			t.Errorf("query %q missing %q", capturedQuery, want)
		}
	}
}

func TestV2RejectsReplacedV1Spellings(t *testing.T) {
	c, err := NewClient("key")
	if err != nil {
		t.Fatal(err)
	}
	for name, params := range map[string]Params{
		"qintitle":         {"qintitle": "x"},
		"domain":           {"domain": "bbc.com"},
		"domainurl":        {"domainurl": "bbc.com"},
		"excludedomain":    {"excludedomain": "bbc.com"},
		"excludedomainurl": {"excludedomainurl": "bbc.com"},
		"prioritydomain":   {"prioritydomain": "top"},
		"full_content":     {"full_content": true},
		"removeduplicate":  {"removeduplicate": true},
		"excludefield":     {"excludefield": "description"},
	} {
		_, err := c.NewsV2(context.Background(), params)
		var vErr *NewsdataValidationError
		if !errors.As(err, &vErr) {
			t.Errorf("%s: expected a validation error, got %v", name, err)
		}
	}
}

func TestV1StillAcceptsV1Spellings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(successBody(`[]`)))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Latest(context.Background(), Params{
		"qintitle":     "x",
		"domain":       "bbc.com",
		"full_content": true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestV2MutexGroups(t *testing.T) {
	c, err := NewClient("key")
	if err != nil {
		t.Fatal(err)
	}
	for name, params := range map[string]Params{
		"source_url":    {"source_url": "a.com", "exclude_source_url": "b.com"},
		"q group":       {"q": "a", "q_in_title": "b"},
		"country group": {"country": "us", "exclude_country": "cn"},
	} {
		_, err := c.NewsV2(context.Background(), params)
		var vErr *NewsdataValidationError
		if !errors.As(err, &vErr) {
			t.Errorf("%s: expected a validation error, got %v", name, err)
		}
	}
}

// The v2 envelope spells the total and cursor differently; Response must
// decode either into TotalResults / NextPage so callers need not branch.
func TestResponseDecodesBothEnvelopeSpellings(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"v1", `{"status":"success","totalResults":42,"results":[],"nextPage":"abc"}`},
		{"v2", `{"status":"success","total_results":42,"results":[],"next_page":"abc"}`},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(tc.body))
		}))

		c, err := NewClient("key", WithBaseURL(srv.URL))
		if err != nil {
			srv.Close()
			t.Fatal(err)
		}
		resp, err := c.NewsV2(context.Background(), Params{"q": "x"})
		srv.Close()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if resp.TotalResults != 42 {
			t.Errorf("%s: TotalResults = %d, want 42", tc.name, resp.TotalResults)
		}
		if resp.NextPage != "abc" {
			t.Errorf("%s: NextPage = %q, want abc", tc.name, resp.NextPage)
		}
	}
}

func TestScrollAllFollowsV2Cursor(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_, _ = w.Write([]byte(
				`{"status":"success","total_results":2,` +
					`"results":[{"article_id":"1"}],"next_page":"p2"}`))
			return
		}
		_, _ = w.Write([]byte(
			`{"status":"success","total_results":2,"results":[{"article_id":"2"}]}`))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL), WithPaginationDelay(0))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.ScrollAll(context.Background(), EndpointNewsV2, Params{"q": "x"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	articles, err := resp.Articles()
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("got %d articles, want 2", len(articles))
	}
	if page != 2 {
		t.Errorf("made %d requests, want 2", page)
	}
}

func TestScrollAllAndPaginateRejectV2Sources(t *testing.T) {
	c, err := NewClient("key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ScrollAll(context.Background(), EndpointSourcesV2, nil, 0); err == nil {
		t.Error("ScrollAll should reject the v2 sources endpoint")
	}
	err = c.Paginate(context.Background(), EndpointSourcesV2, nil,
		func(*Response, error) bool { return true })
	if err == nil {
		t.Error("Paginate should reject the v2 sources endpoint")
	}
}

// ---- the quota split (one error type per 429 credit code) ----------------

func TestQuotaCodesMapToDistinctErrors(t *testing.T) {
	cases := []struct {
		code string
		want func(error) bool
	}{
		{"ApiLimitExceeded", func(err error) bool {
			var e *NewsdataAPILimitExceededError
			return errors.As(err, &e)
		}},
		{"ApiKeyLimitExceeded", func(err error) bool {
			var e *NewsdataAPIKeyLimitExceededError
			return errors.As(err, &e)
		}},
	}
	for _, tc := range cases {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(
				`{"status":"error","results":{"message":"limit","code":"` + tc.code + `"}}`))
		}))

		c, err := NewClient("key", WithBaseURL(srv.URL), WithMaxRetries(5),
			WithRetryBackoff(time.Millisecond), WithRetryBackoffMax(time.Millisecond))
		if err != nil {
			srv.Close()
			t.Fatal(err)
		}
		_, err = c.NewsV2(context.Background(), Params{"q": "x"})
		srv.Close()

		if !tc.want(err) {
			t.Errorf("%s: got %T (%v), want its own error type", tc.code, err, err)
		}
		// Both must still match the base rate-limit error.
		var rl *NewsdataRateLimitError
		if !errors.As(err, &rl) {
			t.Errorf("%s: should still match NewsdataRateLimitError", tc.code)
		}
		if calls != 1 {
			t.Errorf("%s: made %d requests, want 1 (exhausted quota must not retry)",
				tc.code, calls)
		}
	}
}

func TestTransientQuotaCodeStillRetries(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(
			`{"status":"error","results":{"message":"slow down","code":"RateLimitExceeded"}}`))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL), WithMaxRetries(2),
		WithRetryBackoff(time.Millisecond), WithRetryBackoffMax(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.NewsV2(context.Background(), Params{"q": "x"})

	var rl *NewsdataRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("got %T (%v), want NewsdataRateLimitError", err, err)
	}
	var quota *NewsdataAPILimitExceededError
	if errors.As(err, &quota) {
		t.Error("a transient rate limit must not become a quota-exhausted error")
	}
	if calls != 2 {
		t.Errorf("made %d requests, want 2", calls)
	}
}

// ---- excludedomainurl (v1) ----------------------------------------------

func TestExcludeDomainURLIsAV1Filter(t *testing.T) {
	for _, endpoint := range []string{
		"latest", "archive", "crypto", "market",
		"count", "crypto_count", "market_count",
	} {
		if !filters[endpoint]["excludedomainurl"] {
			t.Errorf("%s should accept excludedomainurl", endpoint)
		}
	}
	if filters["sources"]["excludedomainurl"] {
		t.Error("sources should not accept excludedomainurl")
	}
}

func TestExcludeDomainURLJoinsDomainMutexGroup(t *testing.T) {
	c, err := NewClient("key")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Latest(context.Background(), Params{
		"domain":           "a.com",
		"excludedomainurl": "b.com",
	})
	var vErr *NewsdataValidationError
	if !errors.As(err, &vErr) {
		t.Errorf("expected a validation error, got %v", err)
	}
}
