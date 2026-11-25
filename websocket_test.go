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

	"github.com/gorilla/websocket"
)

// wsUpgrader accepts the test client's handshake without origin checks.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// wsServer starts a WebSocket test server. handler runs per accepted
// connection. The returned URL uses the ws:// scheme.
func wsServer(t *testing.T, handler func(*websocket.Conn, *http.Request)) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn, r)
	}))
	return "ws" + strings.TrimPrefix(srv.URL, "http"), srv.Close
}

func wsTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient("key")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestWebSocketStreamsResponses(t *testing.T) {
	url, stop := wsServer(t, func(conn *websocket.Conn, _ *http.Request) {
		_ = conn.WriteMessage(websocket.TextMessage,
			[]byte(`{"status":"success","total_results":1,"results":[{"id":"a1","title":"one"}]}`))
		_ = conn.WriteMessage(websocket.TextMessage,
			[]byte(`{"status":"success","total_results":1,"results":[{"id":"a2","title":"two"}]}`))
	})
	defer stop()

	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url), WithWSReconnect(false))

	var titles []string
	err := ws.Stream(context.Background(), "reg-1", func(r *Response) error {
		arts, err := r.Articles()
		if err != nil {
			return err
		}
		titles = append(titles, arts[0].Title)
		if len(titles) == 2 {
			return ErrStopStream
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Stream returned %v, want nil", err)
	}
	if strings.Join(titles, ",") != "one,two" {
		t.Errorf("titles = %v, want [one two]", titles)
	}
}

// The apikey and registration_id ride on the query string.
func TestWebSocketSendsCredentialsInQuery(t *testing.T) {
	got := make(chan string, 1)
	url, stop := wsServer(t, func(conn *websocket.Conn, r *http.Request) {
		got <- r.URL.RawQuery
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"success","results":[]}`))
	})
	defer stop()

	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url), WithWSReconnect(false))
	_ = ws.Stream(context.Background(), "reg-42", func(*Response) error { return ErrStopStream })

	query := <-got
	if !strings.Contains(query, "apikey=key") {
		t.Errorf("query %q missing apikey", query)
	}
	if !strings.Contains(query, "registration_id=reg-42") {
		t.Errorf("query %q missing registration_id", query)
	}
}

func TestWebSocketSkipsMalformedFrames(t *testing.T) {
	url, stop := wsServer(t, func(conn *websocket.Conn, _ *http.Request) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`not json at all`))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"success","results":[]}`))
	})
	defer stop()

	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url), WithWSReconnect(false))

	var calls int
	_ = ws.Stream(context.Background(), "reg-1", func(*Response) error {
		calls++
		return ErrStopStream
	})
	if calls != 1 {
		t.Errorf("handler called %d times, want 1 (malformed frame should be skipped)", calls)
	}
}

// Handshake 401 is permanent: no reconnect, and a typed auth error.
func TestWebSocketHandshake401IsPermanent(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	// reconnect stays ON to prove a permanent rejection is not retried.
	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url))
	err := ws.Stream(context.Background(), "reg-1", func(*Response) error { return nil })

	var authErr *NewsdataWebSocketAuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("error = %v (%T), want *NewsdataWebSocketAuthError", err, err)
	}
	if n := atomic.LoadInt32(&attempts); n != 1 {
		t.Errorf("connected %d times, want 1 — a permanent rejection must not retry", n)
	}
}

// Close code 1008 (policy violation) is permanent too.
func TestWebSocketPolicyViolationCloseIsPermanent(t *testing.T) {
	var attempts int32
	url, stop := wsServer(t, func(conn *websocket.Conn, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "quota exhausted"),
			time.Now().Add(time.Second),
		)
	})
	defer stop()

	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url))
	err := ws.Stream(context.Background(), "reg-1", func(*Response) error { return nil })

	var authErr *NewsdataWebSocketAuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("error = %v (%T), want *NewsdataWebSocketAuthError", err, err)
	}
	if !strings.Contains(err.Error(), "quota exhausted") {
		t.Errorf("error %q should carry the close reason", err.Error())
	}
	if n := atomic.LoadInt32(&attempts); n != 1 {
		t.Errorf("connected %d times, want 1", n)
	}
}

// A 500 handshake is transient: with reconnect off it surfaces as a plain
// websocket error, not an auth error.
func TestWebSocketTransientHandshakeStopsWhenReconnectDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url), WithWSReconnect(false))
	err := ws.Stream(context.Background(), "reg-1", func(*Response) error { return nil })

	var authErr *NewsdataWebSocketAuthError
	if errors.As(err, &authErr) {
		t.Fatalf("500 handshake should be transient, got auth error: %v", err)
	}
	var wsErr *NewsdataWebSocketError
	if !errors.As(err, &wsErr) {
		t.Fatalf("error = %v (%T), want *NewsdataWebSocketError", err, err)
	}
	if !errors.Is(err, ErrWebSocket) {
		t.Error("error should satisfy errors.Is(err, ErrWebSocket)")
	}
}

// A transient drop reconnects when reconnect is enabled.
func TestWebSocketReconnectsAfterTransientDrop(t *testing.T) {
	var attempts int32
	url, stop := wsServer(t, func(conn *websocket.Conn, _ *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			// Drop abruptly without a close frame — a transient failure.
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"success","results":[]}`))
		time.Sleep(50 * time.Millisecond)
	})
	defer stop()

	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url),
		WithWSReconnectDelay(time.Millisecond, 5*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ws.Stream(ctx, "reg-1", func(*Response) error { return ErrStopStream })
	if err != nil {
		t.Fatalf("Stream returned %v, want nil after reconnect", err)
	}
	if n := atomic.LoadInt32(&attempts); n < 2 {
		t.Errorf("connected %d times, want >= 2 (should reconnect)", n)
	}
}

// Cancelling the context ends the stream with the context error.
func TestWebSocketContextCancelStops(t *testing.T) {
	url, stop := wsServer(t, func(conn *websocket.Conn, _ *http.Request) {
		time.Sleep(2 * time.Second) // hold the connection open
	})
	defer stop()

	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url))
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := ws.Stream(ctx, "reg-1", func(*Response) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

// A handler error propagates unchanged.
func TestWebSocketHandlerErrorPropagates(t *testing.T) {
	url, stop := wsServer(t, func(conn *websocket.Conn, _ *http.Request) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"success","results":[]}`))
		time.Sleep(100 * time.Millisecond)
	})
	defer stop()

	sentinel := errors.New("caller stopped")
	ws := NewWebSocket(wsTestClient(t), WithWSBaseURL(url))
	err := ws.Stream(context.Background(), "reg-1", func(*Response) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want the handler's error", err)
	}
}

func TestWebSocketRejectsEmptyRegistrationID(t *testing.T) {
	ws := NewWebSocket(wsTestClient(t))
	err := ws.Stream(context.Background(), "", func(*Response) error { return nil })
	if !errors.Is(err, ErrValidation) {
		t.Errorf("error = %v, want a validation error", err)
	}
}

func TestWebSocketNextDelayDoublesAndCaps(t *testing.T) {
	ws := NewWebSocket(wsTestClient(t),
		WithWSReconnectDelay(time.Second, 4*time.Second))
	got := []time.Duration{}
	d := time.Second
	for i := 0; i < 4; i++ {
		d = ws.nextDelay(d)
		got = append(got, d)
	}
	want := []time.Duration{2 * time.Second, 4 * time.Second, 4 * time.Second, 4 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("delay[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

// ---- query management -------------------------------------------------

func TestWebSocketRegisterPostsWithNewsType(t *testing.T) {
	var method, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, query = r.Method, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"status":"success","results":{"registration_id":"reg-9"}}`))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.WebSocketRegister(context.Background(), Params{"q": "bitcoin"})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost {
		t.Errorf("method = %s, want POST", method)
	}
	if !strings.Contains(query, "news_type=news") {
		t.Errorf("query %q should carry news_type=news", query)
	}
	if !strings.Contains(query, "q=bitcoin") {
		t.Errorf("query %q should carry the filter", query)
	}
	agg, err := resp.Aggregate()
	if err != nil {
		t.Fatal(err)
	}
	if agg["registration_id"] != "reg-9" {
		t.Errorf("registration_id = %v, want reg-9", agg["registration_id"])
	}
}

// WebSocketRegister must not mutate the caller's Params.
func TestWebSocketRegisterDoesNotMutateCallerParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","results":{}}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL))
	params := Params{"q": "bitcoin"}
	if _, err := c.WebSocketRegister(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if _, ok := params["news_type"]; ok {
		t.Error("WebSocketRegister leaked news_type into the caller's Params")
	}
}

func TestWebSocketFetchUsesGet(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"status":"success","total_results":0,"results":[]}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL))
	if _, err := c.WebSocketFetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet {
		t.Errorf("method = %s, want GET", method)
	}
	if !strings.HasSuffix(path, "/websocket/fetch") {
		t.Errorf("path = %s, want .../websocket/fetch", path)
	}
}

func TestWebSocketDeleteUsesDelete(t *testing.T) {
	var method, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, query = r.Method, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"status":"success","results":{"deleted":true}}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL))
	if _, err := c.WebSocketDelete(context.Background(), "reg-9"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", method)
	}
	if !strings.Contains(query, "registration_id=reg-9") {
		t.Errorf("query %q should carry registration_id", query)
	}
}

func TestWebSocketDeleteRejectsEmptyID(t *testing.T) {
	c, _ := NewClient("key")
	if _, err := c.WebSocketDelete(context.Background(), ""); !errors.Is(err, ErrValidation) {
		t.Errorf("error = %v, want a validation error", err)
	}
}

// A success envelope with no `results` field still succeeds on the
// websocket endpoints (the news endpoints keep requiring it).
func TestWebSocketManagementAcceptsResultlessSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer srv.Close()

	c, _ := NewClient("key", WithBaseURL(srv.URL))
	if _, err := c.WebSocketDelete(context.Background(), "reg-9"); err != nil {
		t.Errorf("resultless success should not error, got %v", err)
	}
}

// ---- API version 2 -------------------------------------------------------
// The real-time service moved to API version 2: the management endpoints sit
// under /2/, news_type names the v2 news feed, and a registration takes the v2
// filter vocabulary only.

func TestWebSocketEndpointsAreUnderAPIVersion2(t *testing.T) {
	for key, want := range map[string]string{
		"websocket_register": "2/websocket/register",
		"websocket_fetch":    "2/websocket/fetch",
		"websocket_delete":   "2/websocket/delete",
	} {
		if got := endpoints[key]; got != want {
			t.Errorf("endpoints[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestWSNewsTypeNamesTheV2NewsFeed(t *testing.T) {
	if wsNewsType != "news" {
		t.Errorf("wsNewsType = %q, want news", wsNewsType)
	}
}

func TestWebSocketRegisterTakesTheV2FilterNames(t *testing.T) {
	set := filters["websocket_register"]
	for _, name := range []string{
		"q_in_title", "q_in_meta", "exclude_country", "exclude_category",
		"exclude_language", "source_url", "exclude_source_url",
		"source_priority", "content", "remove_duplicate", "exclude_field",
	} {
		if !set[name] {
			t.Errorf("websocket_register should accept %s", name)
		}
	}
	for _, name := range []string{
		"qintitle", "qinmeta", "excludecountry", "excludecategory",
		"excludelanguage", "domain", "domainurl", "excludedomain",
		"excludedomainurl", "prioritydomain", "full_content",
		"removeduplicate", "excludefield",
	} {
		if set[name] {
			t.Errorf("websocket_register should no longer accept %s", name)
		}
	}
	// Still no date or paging filters on a standing query.
	for _, name := range []string{"from_date", "to_date", "page", "size", "sort"} {
		if set[name] {
			t.Errorf("websocket_register should not accept %s", name)
		}
	}
}

func TestWebSocketRegisterRejectsAV1FilterName(t *testing.T) {
	c, err := NewClient("key")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.WebSocketRegister(context.Background(), Params{"qintitle": "pizza"})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v (%T), want *NewsdataValidationError", err, err)
	}
}

func TestWebSocketRegisterKeepsTheV2SourceURLMutexGroup(t *testing.T) {
	c, err := NewClient("key")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.WebSocketRegister(context.Background(), Params{
		"source_url":         "a.com",
		"exclude_source_url": "b.com",
	})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v (%T), want *NewsdataValidationError", err, err)
	}
}

// RegistrationLimitExceeded means every registration slot the plan allows is
// in use. It is never retried, but keeps the base error: it is about slots,
// not credits, so there is no dedicated type.
func TestRegistrationLimitExceededKeepsTheBaseError(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(
			`{"status":"error","results":{"message":"limit","code":"RegistrationLimitExceeded"}}`))
	}))
	defer srv.Close()

	c, err := NewClient("key", WithBaseURL(srv.URL), WithMaxRetries(5),
		WithRetryBackoff(time.Millisecond), WithRetryBackoffMax(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.WebSocketRegister(context.Background(), Params{"q": "x"})

	var rl *NewsdataRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error = %v (%T), want *NewsdataRateLimitError", err, err)
	}
	var apiLimit *NewsdataAPILimitExceededError
	var keyLimit *NewsdataAPIKeyLimitExceededError
	if errors.As(err, &apiLimit) || errors.As(err, &keyLimit) {
		t.Errorf("error = %T, want the plain rate-limit error", err)
	}
	if calls != 1 {
		t.Errorf("made %d requests, want 1 — a full registration set must not retry", calls)
	}
}
