package newsdataapi

import (
	"errors"
	"fmt"
)

// All errors returned by the SDK derive from NewsdataError, so callers can use
//
//	var ndErr *newsdataapi.NewsdataError
//	if errors.As(err, &ndErr) { ... }
//
// as a catch-all. More specific subclasses are provided for cases where
// callers want to react differently (validation, auth, rate limiting, etc.).

// NewsdataError is the base type wrapping every SDK-originated error.
type NewsdataError struct {
	Op  string // optional: the API operation, e.g. "latest"
	Msg string
	Err error
}

func (e *NewsdataError) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("newsdataapi: %s: %s", e.Op, e.Msg)
	}
	return "newsdataapi: " + e.Msg
}

func (e *NewsdataError) Unwrap() error { return e.Err }

// newError constructs a NewsdataError.
func newError(op, msg string, err error) *NewsdataError {
	return &NewsdataError{Op: op, Msg: msg, Err: err}
}

// NewsdataValidationError indicates a parameter failed client-side validation.
// No request was sent.
type NewsdataValidationError struct {
	Param   string // The offending parameter name, when known.
	Message string
}

func (e *NewsdataValidationError) Error() string {
	if e.Param != "" {
		return fmt.Sprintf("newsdataapi: invalid param %q: %s", e.Param, e.Message)
	}
	return "newsdataapi: " + e.Message
}

// NewsdataAPIError indicates the API returned a structured error response.
type NewsdataAPIError struct {
	StatusCode   int
	Message      string
	ResponseBody []byte
}

func (e *NewsdataAPIError) Error() string {
	return fmt.Sprintf("newsdataapi: API error %d: %s", e.StatusCode, e.Message)
}

// NewsdataAuthError is raised on 401 / 403 responses.
type NewsdataAuthError struct{ *NewsdataAPIError }

func (e *NewsdataAuthError) Error() string {
	return fmt.Sprintf("newsdataapi: auth error %d: %s", e.StatusCode, e.Message)
}

// Unwrap allows `errors.As` to also match NewsdataAPIError on this type.
func (e *NewsdataAuthError) Unwrap() error { return e.NewsdataAPIError }

// NewsdataRateLimitError is raised on 429 responses once retries are exhausted.
type NewsdataRateLimitError struct {
	*NewsdataAPIError
	RetryAfter int // seconds, when the Retry-After header was parseable; 0 otherwise
}

func (e *NewsdataRateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("newsdataapi: rate limited (retry after %ds): %s", e.RetryAfter, e.Message)
	}
	return fmt.Sprintf("newsdataapi: rate limited: %s", e.Message)
}

func (e *NewsdataRateLimitError) Unwrap() error { return e.NewsdataAPIError }

// NewsdataServerError is raised on 5xx responses once retries are exhausted.
type NewsdataServerError struct{ *NewsdataAPIError }

func (e *NewsdataServerError) Error() string {
	return fmt.Sprintf("newsdataapi: server error %d: %s", e.StatusCode, e.Message)
}

func (e *NewsdataServerError) Unwrap() error { return e.NewsdataAPIError }

// NewsdataNetworkError indicates a transport-level failure (DNS, TLS, timeout,
// context cancellation) prevented the request from completing.
type NewsdataNetworkError struct {
	Err error
}

func (e *NewsdataNetworkError) Error() string {
	return "newsdataapi: network error: " + e.Err.Error()
}

func (e *NewsdataNetworkError) Unwrap() error { return e.Err }

// Sentinel for callers that want a single error.Is check.
var (
	ErrValidation = errors.New("newsdataapi: validation error")
	ErrAPI        = errors.New("newsdataapi: API error")
	ErrNetwork    = errors.New("newsdataapi: network error")
)

// Is hooks so callers can use errors.Is(err, ErrValidation) etc.
func (e *NewsdataValidationError) Is(target error) bool { return target == ErrValidation }
func (e *NewsdataAPIError) Is(target error) bool        { return target == ErrAPI }
func (e *NewsdataNetworkError) Is(target error) bool    { return target == ErrNetwork }
