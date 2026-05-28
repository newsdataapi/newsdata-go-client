package newsdataapi

import (
	"net/http"
	"time"
)

// Logger is the minimal interface the client uses for diagnostic logs. It is
// compatible with anything that has Info / Warn methods (slog.Logger does, and
// any custom logger can be adapted with a thin wrapper).
type Logger interface {
	Info(msg string)
	Warn(msg string)
}

// Option configures a Client at construction time.
type Option func(*Client) error

// WithBaseURL overrides the API base URL (useful for staging / a proxy).
// The trailing slash is added if missing.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return &NewsdataValidationError{Param: "baseURL", Message: "must be a non-empty string"}
		}
		if u[len(u)-1] != '/' {
			u += "/"
		}
		c.baseURL = u
		return nil
	}
}

// WithTimeout sets a per-request timeout. Applied via http.Client.Timeout, so
// it covers the full request (connect + body). Default: 30s.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) error {
		c.timeout = d
		return nil
	}
}

// WithMaxRetries sets the total number of request attempts (1 = no retry).
// Retries cover network errors, HTTP 429, and 5xx responses. Auth and other
// 4xx errors are never retried. Default: 5.
func WithMaxRetries(n int) Option {
	return func(c *Client) error {
		if n < 1 {
			n = 1
		}
		c.maxRetries = n
		return nil
	}
}

// WithRetryBackoff sets the base for exponential backoff between retries.
// Default: 2s. Successive attempts wait 2s, 4s, 8s, 16s, 32s (capped by
// WithRetryBackoffMax).
func WithRetryBackoff(d time.Duration) Option {
	return func(c *Client) error {
		c.retryBackoff = d
		return nil
	}
}

// WithRetryBackoffMax caps the longest single backoff sleep. Default: 60s.
func WithRetryBackoffMax(d time.Duration) Option {
	return func(c *Client) error {
		c.retryBackoffMax = d
		return nil
	}
}

// WithPaginationDelay sets the delay between page requests when using
// ScrollAll or Paginate. Default: 1s.
func WithPaginationDelay(d time.Duration) Option {
	return func(c *Client) error {
		c.paginationDelay = d
		return nil
	}
}

// WithHTTPClient injects a custom *http.Client (e.g. with a custom Transport
// for proxies, mTLS, instrumentation). Note: setting this overrides the
// WithTimeout value — set the timeout on your own client instead.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return &NewsdataValidationError{Param: "httpClient", Message: "must be non-nil"}
		}
		c.httpClient = hc
		c.userSuppliedHTTP = true
		return nil
	}
}

// WithLogger attaches a logger that receives diagnostic messages (one line per
// request attempt, retry, and backoff). The API key is always redacted from
// logged URLs.
func WithLogger(l Logger) Option {
	return func(c *Client) error {
		c.logger = l
		return nil
	}
}

// WithIncludeHeaders, when true, attaches the response headers on each
// returned Response (as Response.ResponseHeaders). Default: false.
func WithIncludeHeaders(b bool) Option {
	return func(c *Client) error {
		c.includeHeaders = b
		return nil
	}
}
