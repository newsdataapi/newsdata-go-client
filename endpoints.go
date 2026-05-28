package newsdataapi

import (
	"context"
	"encoding/json"
)

// Latest fetches real-time news. GET /1/latest.
func (c *Client) Latest(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointLatest, params)
}

// Archive fetches historical news. GET /1/archive.
func (c *Client) Archive(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointArchive, params)
}

// Crypto fetches cryptocurrency news. GET /1/crypto.
func (c *Client) Crypto(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointCrypto, params)
}

// Sources lists available news sources. GET /1/sources. Single-page endpoint;
// ScrollAll and Paginate are not supported for this one.
func (c *Client) Sources(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointSources, params)
}

// Market fetches market / financial news. GET /1/market.
func (c *Client) Market(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointMarket, params)
}

// Count fetches aggregate news counts for a date range. GET /1/count.
// Requires "from_date" and "to_date" in params.
func (c *Client) Count(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointCount, params)
}

// CryptoCount fetches aggregate crypto counts. GET /1/crypto/count.
// Requires "from_date" and "to_date" in params.
func (c *Client) CryptoCount(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointCryptoCount, params)
}

// MarketCount fetches aggregate market counts. GET /1/market/count.
// Requires "from_date" and "to_date" in params.
func (c *Client) MarketCount(ctx context.Context, params Params) (*Response, error) {
	return c.do(ctx, EndpointMarketCount, params)
}

// ScrollAll follows nextPage cursors and returns one merged Response, capped
// at maxResults articles (0 = no cap, follow until exhaustion).
//
// For news endpoints, Results is the concatenation of every page's results.
// For count endpoints (where the final page returns an aggregate object), the
// aggregate is placed in the returned Response as Results and the article
// arrays are concatenated separately (see Articles() / Aggregate()).
//
// endpoint must be one of the Endpoint* constants. Sources is not supported.
func (c *Client) ScrollAll(ctx context.Context, endpoint string, params Params, maxResults int) (*Response, error) {
	if endpoint == EndpointSources {
		return nil, &NewsdataValidationError{
			Message: "ScrollAll is not supported for the sources endpoint",
		}
	}

	// Copy params so we can override "page" without mutating the caller's map.
	req := make(Params, len(params)+1)
	for k, v := range params {
		req[k] = v
	}

	var (
		merged   = []json.RawMessage{}
		total    int
		nextPage string
		lastResp *Response
	)

	for {
		resp, err := c.do(ctx, endpoint, req)
		if err != nil {
			return nil, err
		}
		lastResp = resp
		total = resp.TotalResults

		// News pages: Results is an array → split and append items.
		// Count final page: Results is an object → keep as-is in lastResp.
		if items, err := splitJSONArray(resp.Results); err == nil && items != nil {
			merged = append(merged, items...)
		}

		nextPage = resp.NextPage
		if maxResults > 0 && len(merged) >= maxResults {
			merged = merged[:maxResults]
			nextPage = ""
		}
		if nextPage == "" {
			break
		}
		req["page"] = nextPage
		if !sleepCtx(ctx, c.paginationDelay) {
			return nil, &NewsdataNetworkError{Err: ctx.Err()}
		}
	}

	out := &Response{
		Status:       "success",
		TotalResults: total,
		NextPage:     "",
	}
	if len(merged) > 0 {
		// Re-assemble as a JSON array.
		joined, err := joinJSONArray(merged)
		if err != nil {
			return nil, &NewsdataAPIError{Message: "could not merge pages: " + err.Error()}
		}
		out.Results = joined
	} else if lastResp != nil {
		// No array results were accumulated — e.g. count endpoint's aggregate.
		out.Results = lastResp.Results
	}
	if c.includeHeaders && lastResp != nil {
		out.ResponseHeaders = lastResp.ResponseHeaders
	}
	return out, nil
}

// Paginate calls the endpoint once per page, invoking yield on each. Return
// false from yield to stop early; the function returns the error yield was
// called with (if any) or nil on clean completion.
//
// endpoint must be one of the Endpoint* constants. Sources is not supported.
func (c *Client) Paginate(
	ctx context.Context,
	endpoint string,
	params Params,
	yield func(*Response, error) bool,
) error {
	if endpoint == EndpointSources {
		return &NewsdataValidationError{
			Message: "Paginate is not supported for the sources endpoint",
		}
	}

	req := make(Params, len(params)+1)
	for k, v := range params {
		req[k] = v
	}

	for {
		resp, err := c.do(ctx, endpoint, req)
		cont := yield(resp, err)
		if err != nil {
			return err
		}
		if !cont {
			return nil
		}
		// On count endpoints the final page returns an object (not an array).
		// Stop after yielding it.
		if resp.Results != nil && len(resp.Results) > 0 && resp.Results[0] == '{' {
			return nil
		}
		if resp.NextPage == "" {
			return nil
		}
		req["page"] = resp.NextPage
		if !sleepCtx(ctx, c.paginationDelay) {
			return &NewsdataNetworkError{Err: ctx.Err()}
		}
	}
}

// splitJSONArray decodes a top-level JSON array into its element RawMessages.
// Returns (nil, nil) if data is empty or not an array.
func splitJSONArray(data json.RawMessage) ([]json.RawMessage, error) {
	if len(data) == 0 || data[0] != '[' {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// joinJSONArray re-serializes a slice of RawMessages as a JSON array.
func joinJSONArray(items []json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(items)
}
