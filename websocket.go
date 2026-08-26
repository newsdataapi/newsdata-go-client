package newsdataapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket streams the news matching a registered real-time query.
//
// Register the query first with Client.WebSocketRegister (or list existing
// ones with Client.WebSocketFetch), then pass its registration_id to Stream:
//
//	ws := newsdataapi.NewWebSocket(client)
//	err := ws.Stream(ctx, registrationID, func(resp *newsdataapi.Response) error {
//		articles, _ := resp.Articles()
//		for _, a := range articles {
//			fmt.Println(a.Title)
//		}
//		return nil
//	})
//
// Returning a non-nil error from the handler stops the stream and Stream
// returns that error unchanged; return ErrStopStream to stop without one.
//
// Transient drops (network errors, server restarts, abnormal closes) are
// reconnected automatically with a capped exponential backoff. Pass
// WithWSReconnect(false) to stop on the first disconnect instead. A permanent
// rejection always returns *NewsdataWebSocketAuthError and is never retried.
//
// A single Stream call must not be shared between goroutines; separate Stream
// calls on the same WebSocket are independent.
type WebSocket struct {
	client            *Client
	baseURL           string
	reconnect         bool
	reconnectDelay    time.Duration
	reconnectDelayMax time.Duration
	handshakeTimeout  time.Duration
	pingInterval      time.Duration
	pongTimeout       time.Duration
	headers           http.Header
	proxy             func(*http.Request) (*url.URL, error)
}

// ErrStopStream stops a Stream cleanly from inside its handler. Stream returns
// nil when the handler returns it.
var ErrStopStream = errors.New("newsdataapi: stop stream")

// WSOption configures a WebSocket. Pass to NewWebSocket.
type WSOption func(*WebSocket)

// WithWSBaseURL overrides the WebSocket endpoint. Defaults to
// wss://ws.newsdata.io/ws/event; override for staging, self-hosted, or
// proxied environments.
func WithWSBaseURL(u string) WSOption {
	return func(w *WebSocket) { w.baseURL = u }
}

// WithWSReconnect enables or disables automatic reconnection on transient
// drops. Default true; when false, Stream returns on the first disconnect.
func WithWSReconnect(on bool) WSOption {
	return func(w *WebSocket) { w.reconnect = on }
}

// WithWSReconnectDelay sets the wait before the first reconnect (it doubles
// after each consecutive failure) and the upper bound on that delay.
func WithWSReconnectDelay(delay, max time.Duration) WSOption {
	return func(w *WebSocket) { w.reconnectDelay, w.reconnectDelayMax = delay, max }
}

// WithWSHandshakeTimeout bounds the opening handshake. Zero disables it.
func WithWSHandshakeTimeout(d time.Duration) WSOption {
	return func(w *WebSocket) { w.handshakeTimeout = d }
}

// WithWSKeepalive sets the interval between keepalive pings and how long to
// wait for the pong reply before treating the connection as dead. A zero
// interval disables keepalive.
func WithWSKeepalive(interval, timeout time.Duration) WSOption {
	return func(w *WebSocket) { w.pingInterval, w.pongTimeout = interval, timeout }
}

// WithWSHeaders adds extra HTTP headers to the opening handshake.
func WithWSHeaders(h http.Header) WSOption {
	return func(w *WebSocket) { w.headers = h }
}

// WithWSProxy routes the WebSocket connection through a proxy.
func WithWSProxy(f func(*http.Request) (*url.URL, error)) WSOption {
	return func(w *WebSocket) { w.proxy = f }
}

// NewWebSocket constructs a WebSocket bound to client, which supplies the API
// key. client is not closed by the returned WebSocket.
func NewWebSocket(client *Client, opts ...WSOption) *WebSocket {
	w := &WebSocket{
		client:            client,
		baseURL:           wsBaseURL,
		reconnect:         true,
		reconnectDelay:    defaultWSReconnectDelay,
		reconnectDelayMax: defaultWSReconnectDelayMax,
		handshakeTimeout:  defaultWSHandshakeTimeout,
		pingInterval:      defaultWSPingInterval,
		pongTimeout:       defaultWSPongTimeout,
		proxy:             http.ProxyFromEnvironment,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Register registers a real-time query, delegating to the wrapped Client. See
// Client.WebSocketRegister.
func (w *WebSocket) Register(ctx context.Context, params Params) (*Response, error) {
	return w.client.WebSocketRegister(ctx, params)
}

// Fetch lists the account's registered real-time queries, delegating to the
// wrapped Client. See Client.WebSocketFetch.
func (w *WebSocket) Fetch(ctx context.Context) (*Response, error) {
	return w.client.WebSocketFetch(ctx)
}

// Delete removes a registered real-time query, delegating to the wrapped
// Client. See Client.WebSocketDelete.
func (w *WebSocket) Delete(ctx context.Context, registrationID string) (*Response, error) {
	return w.client.WebSocketDelete(ctx, registrationID)
}

func (w *WebSocket) url(registrationID string) string {
	v := url.Values{}
	v.Set("apikey", w.client.apiKey)
	v.Set("registration_id", registrationID)
	return w.baseURL + "?" + v.Encode()
}

// nextDelay doubles delay, capped at reconnectDelayMax.
func (w *WebSocket) nextDelay(delay time.Duration) time.Duration {
	d := delay * 2
	if d > w.reconnectDelayMax || d <= 0 {
		return w.reconnectDelayMax
	}
	return d
}

// permanentAuthError returns the error to surface if err is a permanent
// rejection, else nil (the caller then treats it as transient and reconnects).
//
// The server always accepts the handshake and then closes with code 1008 on a
// permanent failure — "invalid credentials or registration not found" (bad
// apikey/registration_id, or the plan lacks WebSocket access), "api limit
// reached" (no credits left), or "device limit reached" (more than the allowed
// simultaneous devices for one registration_id, default 5). Every other close
// code — including 1013 "send timeout", which means this client read too
// slowly — is transient and reconnects.
//
// The handshake-status check below is defensive: the documented server never
// rejects the handshake itself, but a proxy in front of it might.
//
// resp is the handshake response, non-nil only for websocket.ErrBadHandshake.
func permanentAuthError(err error, resp *http.Response) *NewsdataWebSocketAuthError {
	if errors.Is(err, websocket.ErrBadHandshake) {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized ||
			resp.StatusCode == http.StatusForbidden) {
			return &NewsdataWebSocketAuthError{
				&NewsdataWebSocketError{Message: "connection rejected", Err: err},
			}
		}
		return nil
	}
	var ce *websocket.CloseError
	if errors.As(err, &ce) && ce.Code == wsPolicyViolation {
		msg := ce.Text
		if msg == "" {
			msg = "connection rejected"
		}
		return &NewsdataWebSocketAuthError{
			&NewsdataWebSocketError{Message: msg, Err: err},
		}
	}
	return nil
}

// transientError wraps a transient failure, used only when reconnect is off so
// the caller stops instead of retrying.
func transientError(err error, resp *http.Response) *NewsdataWebSocketError {
	if errors.Is(err, websocket.ErrBadHandshake) && resp != nil {
		return &NewsdataWebSocketError{
			Message: fmt.Sprintf("handshake failed (HTTP %d)", resp.StatusCode),
			Err:     err,
		}
	}
	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		return &NewsdataWebSocketError{Message: "connection closed", Err: err}
	}
	return &NewsdataWebSocketError{Message: "connection error: " + err.Error(), Err: err}
}

// Stream connects and calls fn with each response as it arrives. Responses
// have the familiar status / totalResults / results shape.
//
// Stream blocks until ctx is cancelled, fn returns an error, or — with
// reconnect disabled — the connection drops. Cancelling ctx returns
// ctx.Err(); a handler error is returned unchanged, except ErrStopStream
// which returns nil.
func (w *WebSocket) Stream(ctx context.Context, registrationID string, fn func(*Response) error) error {
	if registrationID == "" {
		return &NewsdataValidationError{
			Param: "registration_id", Message: "must be a non-empty string",
		}
	}
	if fn == nil {
		return &NewsdataValidationError{Param: "fn", Message: "must not be nil"}
	}

	rawURL := w.url(registrationID)
	logURL := redactAPIKey(rawURL)
	delay := w.reconnectDelay

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		closedNormally, err := w.runOnce(ctx, rawURL, logURL, fn, &delay)
		if err != nil {
			if errors.Is(err, ErrStopStream) {
				return nil
			}
			return err
		}
		if closedNormally && !w.reconnect {
			return nil
		}

		// Transient failure, or a normal close with reconnect enabled: wait
		// (capped exponential backoff) and reconnect.
		if !sleepCtx(ctx, delay) {
			return ctx.Err()
		}
		delay = w.nextDelay(delay)
	}
}

// runOnce holds a single connection open until it drops or fn stops it. It
// reports whether the connection closed normally, and returns a non-nil error
// only when Stream must stop.
func (w *WebSocket) runOnce(
	ctx context.Context,
	rawURL, logURL string,
	fn func(*Response) error,
	delay *time.Duration,
) (closedNormally bool, stop error) {
	dialer := &websocket.Dialer{
		HandshakeTimeout: w.handshakeTimeout,
		Proxy:            w.proxy,
	}
	conn, resp, err := dialer.DialContext(ctx, rawURL, w.headers)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return false, w.handleFailure(ctx, err, resp, *delay, logURL)
	}
	defer conn.Close()

	w.client.log("info", "connected to "+logURL)
	*delay = w.reconnectDelay // reset after a successful connect

	// No read limit: a single response can carry a full page of articles.
	conn.SetReadLimit(0)

	// Close the connection when ctx is cancelled so the blocking ReadMessage
	// below returns instead of hanging.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()

	w.startKeepalive(conn, done)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return false, ctxErr
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return true, nil
			}
			return false, w.handleFailure(ctx, err, nil, *delay, logURL)
		}

		var parsed Response
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue // skip malformed frames
		}
		if err := fn(&parsed); err != nil {
			return false, err
		}
	}
}

// startKeepalive pings on an interval and fails the read loop if a pong does
// not arrive within pongTimeout. A zero pingInterval disables keepalive (and
// with it the read deadline).
func (w *WebSocket) startKeepalive(conn *websocket.Conn, done <-chan struct{}) {
	if w.pingInterval <= 0 {
		return
	}
	extend := func() {
		if w.pongTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(w.pingInterval + w.pongTimeout))
		}
	}
	extend()
	conn.SetPongHandler(func(string) error {
		extend()
		return nil
	})
	go func() {
		ticker := time.NewTicker(w.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				deadline := time.Now().Add(w.pongTimeout)
				if w.pongTimeout <= 0 {
					deadline = time.Now().Add(w.pingInterval)
				}
				if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
					// The read loop will observe the same failure and reconnect.
					return
				}
			}
		}
	}()
}

// handleFailure classifies a connection failure: permanent rejections and
// (when reconnect is off) transient ones stop the stream; otherwise it logs
// and returns nil so Stream backs off and retries.
func (w *WebSocket) handleFailure(
	ctx context.Context, err error, resp *http.Response, delay time.Duration, logURL string,
) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if auth := permanentAuthError(err, resp); auth != nil {
		return auth
	}
	if !w.reconnect {
		return transientError(err, resp)
	}
	w.client.log("warn", fmt.Sprintf(
		"connection to %s failed (%v); reconnecting in %s", logURL, err, delay))
	return nil
}
