package newsdataapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Endpoint name constants. These are also the strings ScrollAll and Paginate
// accept for their `endpoint` argument.
const (
	EndpointLatest      = "latest"
	EndpointArchive     = "archive"
	EndpointCrypto      = "crypto"
	EndpointSources     = "sources"
	EndpointMarket      = "market"
	EndpointCount       = "count"
	EndpointCryptoCount = "crypto_count"
	EndpointMarketCount = "market_count"

	// Real-time WebSocket query management.
	EndpointWebSocketRegister = "websocket_register"
	EndpointWebSocketFetch    = "websocket_fetch"
	EndpointWebSocketDelete   = "websocket_delete"
)

// Client is the HTTP client for the Newsdata.io API. Construct with NewClient
// and call the per-endpoint methods (Latest, Archive, ...) — those return a
// *Response and any client-side validation error or API-mapped error.
//
// Client is safe for concurrent use by multiple goroutines.
type Client struct {
	apiKey           string
	baseURL          string
	httpClient       *http.Client
	userSuppliedHTTP bool
	timeout          time.Duration
	maxRetries       int
	retryBackoff     time.Duration
	retryBackoffMax  time.Duration
	paginationDelay  time.Duration
	includeHeaders   bool
	logger           Logger
}

// NewClient constructs a Client. apiKey is required; opts override the
// defaults listed in each WithXxx helper.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, &NewsdataValidationError{Param: "apiKey", Message: "must be a non-empty string"}
	}
	c := &Client{
		apiKey:          apiKey,
		baseURL:         baseURL,
		timeout:         defaultRequestTimeout,
		maxRetries:      defaultMaxRetries,
		retryBackoff:    defaultRetryBackoff,
		retryBackoffMax: defaultRetryBackoffMax,
		paginationDelay: defaultPaginationDelay,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: c.timeout}
	} else if !c.userSuppliedHTTP {
		c.httpClient.Timeout = c.timeout
	}
	return c, nil
}

// do is the workhorse: validate + encode params, build the URL with apikey,
// execute with retries and backoff, map response → typed error or *Response.
func (c *Client) do(ctx context.Context, endpoint string, params Params) (*Response, error) {
	path, ok := endpoints[endpoint]
	if !ok {
		return nil, &NewsdataValidationError{Message: "unknown endpoint: " + endpoint}
	}
	values, err := validateAndEncode(endpoint, params)
	if err != nil {
		return nil, err
	}
	values.Set("apikey", c.apiKey)

	method := http.MethodGet
	if m, ok := endpointMethods[endpoint]; ok {
		method = m
	}

	fullURL := c.baseURL + path + "?" + values.Encode()
	logURL := redactAPIKey(fullURL)

	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		c.log("info", fmt.Sprintf("%s %s (attempt %d/%d)", method, logURL, attempt, c.maxRetries))

		req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
		if err != nil {
			return nil, &NewsdataNetworkError{Err: err}
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Don't retry on a cancelled context — the caller asked us to stop.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, &NewsdataNetworkError{Err: err}
			}
			lastErr = &NewsdataNetworkError{Err: err}
			c.log("warn", fmt.Sprintf("network error: %v", err))
			if attempt >= c.maxRetries {
				return nil, lastErr
			}
			if !sleepCtx(ctx, c.backoff(attempt)) {
				return nil, &NewsdataNetworkError{Err: ctx.Err()}
			}
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = &NewsdataNetworkError{Err: readErr}
			if attempt >= c.maxRetries {
				return nil, lastErr
			}
			if !sleepCtx(ctx, c.backoff(attempt)) {
				return nil, &NewsdataNetworkError{Err: ctx.Err()}
			}
			continue
		}

		status := resp.StatusCode

		// Try to decode the body as JSON. If it isn't JSON and the status is
		// 5xx, retry; otherwise wrap as an API error.
		isJSON := len(body) > 0 && (body[0] == '{' || body[0] == '[')
		if !isJSON {
			if status >= 500 && attempt < c.maxRetries {
				c.log("warn", fmt.Sprintf("non-JSON response (status %d)", status))
				if !sleepCtx(ctx, c.backoff(attempt)) {
					return nil, &NewsdataNetworkError{Err: ctx.Err()}
				}
				continue
			}
			return nil, &NewsdataAPIError{
				StatusCode:   status,
				Message:      fmt.Sprintf("non-JSON response from API (status %d)", status),
				ResponseBody: body,
			}
		}

		var parsed Response
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, &NewsdataAPIError{
				StatusCode:   status,
				Message:      "could not decode response: " + err.Error(),
				ResponseBody: body,
			}
		}

		if status == http.StatusOK && parsed.Status == "success" &&
			(len(parsed.Results) > 0 || resultsOptional[endpoint]) {
			if c.includeHeaders {
				parsed.ResponseHeaders = resp.Header.Clone()
			}
			return &parsed, nil
		}

		// Pull a human-readable message out of the body when present.
		message := extractErrorMessage(body, status)

		if status == 429 {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			// A 429 covers three cases: a burst limit, a rate limit, and
			// exhausted API credits. Only the first two are worth retrying —
			// waiting out the backoff cannot conjure more credits.
			if quotaExhausted(body) || attempt >= c.maxRetries {
				return nil, &NewsdataRateLimitError{
					NewsdataAPIError: &NewsdataAPIError{
						StatusCode: 429, Message: message, ResponseBody: body,
					},
					RetryAfter: retryAfter,
				}
			}
			wait := time.Duration(retryAfter) * time.Second
			if wait <= 0 {
				wait = c.backoff(attempt)
			}
			c.log("warn", fmt.Sprintf("429 rate limit; sleeping %s", wait))
			if !sleepCtx(ctx, wait) {
				return nil, &NewsdataNetworkError{Err: ctx.Err()}
			}
			continue
		}

		if status >= 500 {
			lastErr = &NewsdataServerError{
				NewsdataAPIError: &NewsdataAPIError{
					StatusCode: status, Message: message, ResponseBody: body,
				},
			}
			if attempt >= c.maxRetries {
				return nil, lastErr
			}
			c.log("warn", fmt.Sprintf("%d server error", status))
			if !sleepCtx(ctx, c.backoff(attempt)) {
				return nil, &NewsdataNetworkError{Err: ctx.Err()}
			}
			continue
		}

		if status == 401 || status == 403 {
			return nil, &NewsdataAuthError{
				NewsdataAPIError: &NewsdataAPIError{
					StatusCode: status, Message: message, ResponseBody: body,
				},
			}
		}

		// Other 4xx — never retried.
		return nil, &NewsdataAPIError{
			StatusCode: status, Message: message, ResponseBody: body,
		}
	}

	// Defensive: the loop always returns or continues above.
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, newError(endpoint, fmt.Sprintf("did not complete (maxRetries=%d)", c.maxRetries), nil)
}

// backoff returns the wait duration for the given attempt (1-indexed),
// capped at retryBackoffMax.
func (c *Client) backoff(attempt int) time.Duration {
	d := c.retryBackoff << (attempt - 1) // doubles each attempt
	if d > c.retryBackoffMax || d <= 0 {
		return c.retryBackoffMax
	}
	return d
}

func (c *Client) log(level, msg string) {
	if c.logger == nil {
		return
	}
	switch level {
	case "warn":
		c.logger.Warn("[newsdataapi] " + msg)
	default:
		c.logger.Info("[newsdataapi] " + msg)
	}
}

// sleepCtx waits for d, returning false if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// parseRetryAfter parses a Retry-After header (integer seconds or HTTP-date)
// into seconds. Returns 0 when the value can't be parsed.
func parseRetryAfter(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if n, err := strconv.Atoi(value); err == nil {
		if n < 0 {
			return 0
		}
		return n
	}
	if t, err := http.ParseTime(value); err == nil {
		secs := int(time.Until(t).Seconds())
		if secs < 0 {
			return 0
		}
		return secs
	}
	return 0
}

var apiKeyRegexp = regexp.MustCompile(`(?i)(apikey=)[^&]*`)

// redactAPIKey replaces the apikey query parameter's value with REDACTED so
// the URL is safe to log.
func redactAPIKey(rawURL string) string {
	return apiKeyRegexp.ReplaceAllString(rawURL, "${1}REDACTED")
}

// extractErrorMessage pulls a useful "what went wrong" string out of an API
// error body. Falls back to a generic "HTTP <status>" message.
// quotaExhausted reports whether a 429 body carries an error code meaning the
// account is out of API credits, as opposed to a transient rate limit.
func quotaExhausted(body []byte) bool {
	var envelope struct {
		Results struct {
			Code string `json:"code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	return quotaExhaustedCodes[envelope.Results.Code]
}

func extractErrorMessage(body []byte, status int) string {
	var envelope struct {
		Results struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"results"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Results.Message != "" {
			return envelope.Results.Message
		}
		if envelope.Message != "" {
			return envelope.Message
		}
	}
	return fmt.Sprintf("API request failed with HTTP %d", status)
}
