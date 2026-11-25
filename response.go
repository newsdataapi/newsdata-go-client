package newsdataapi

import (
	"encoding/json"
	"net/http"
)

// Response is the top-level JSON envelope returned by every endpoint.
//
// `Results` is held as raw JSON because its type varies by endpoint:
//   - news endpoints (latest, archive, crypto, market) return an array of articles;
//   - the count endpoints return an aggregate object on the final page.
//
// Use the Articles() / ArticlesV2() / Aggregate() helpers to decode it into a
// typed shape. Articles() decodes the v1 article layout, ArticlesV2() the v2
// one; the real-time stream carries v2 articles.
//
// v1 and v2 spell the envelope's total and cursor differently
// (totalResults / nextPage vs total_results / next_page). UnmarshalJSON
// accepts either, so TotalResults and NextPage are populated whichever
// version answered and callers need not branch on it.
type Response struct {
	Status          string          `json:"status,omitempty"`
	TotalResults    int             `json:"totalResults,omitempty"`
	Results         json.RawMessage `json:"results,omitempty"`
	NextPage        string          `json:"nextPage,omitempty"`
	ResponseHeaders http.Header     `json:"-"`
}

// UnmarshalJSON decodes either the v1 or the v2 envelope spelling. The v2
// keys win when both are present, which the API never does.
func (r *Response) UnmarshalJSON(data []byte) error {
	var raw struct {
		Status         string          `json:"status"`
		TotalResults   *int            `json:"totalResults"`
		TotalResultsV2 *int            `json:"total_results"`
		Results        json.RawMessage `json:"results"`
		NextPage       *string         `json:"nextPage"`
		NextPageV2     *string         `json:"next_page"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Status = raw.Status
	r.Results = raw.Results

	r.TotalResults = 0
	if raw.TotalResults != nil {
		r.TotalResults = *raw.TotalResults
	}
	if raw.TotalResultsV2 != nil {
		r.TotalResults = *raw.TotalResultsV2
	}

	r.NextPage = ""
	if raw.NextPage != nil {
		r.NextPage = *raw.NextPage
	}
	if raw.NextPageV2 != nil {
		r.NextPage = *raw.NextPageV2
	}

	return nil
}

// Articles decodes Results as []Article. Returns nil, nil when Results is
// empty or not an array (e.g. on a count endpoint's aggregate page).
func (r *Response) Articles() ([]Article, error) {
	if len(r.Results) == 0 || r.Results[0] != '[' {
		return nil, nil
	}
	var out []Article
	if err := json.Unmarshal(r.Results, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ArticlesV2 decodes Results as []ArticleV2, the article layout of API v2 and
// of the real-time stream. Returns nil, nil when Results is empty or not an
// array (e.g. on a count endpoint's aggregate page).
//
// Use Articles() instead for the v1 endpoints, whose articles spell the same
// data differently (link, pubDate, article_id).
func (r *Response) ArticlesV2() ([]ArticleV2, error) {
	if len(r.Results) == 0 || r.Results[0] != '[' {
		return nil, nil
	}
	var out []ArticleV2
	if err := json.Unmarshal(r.Results, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Aggregate decodes Results as a map (the shape count endpoints return).
// Returns nil, nil when Results is empty or not an object.
func (r *Response) Aggregate() (map[string]any, error) {
	if len(r.Results) == 0 || r.Results[0] != '{' {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal(r.Results, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Article is the typed shape of a single news result from API v1. Fields use
// snake_case JSON tags to match the API; AI-enriched fields (ai_*) are set on
// articles where the platform produced them and are otherwise empty.
//
// v2 renamed and regrouped these fields; see ArticleV2.
type Article struct {
	ArticleID      string         `json:"article_id"`
	Title          string         `json:"title"`
	Link           string         `json:"link"`
	Description    string         `json:"description,omitempty"`
	Content        string         `json:"content,omitempty"`
	Keywords       []string       `json:"keywords,omitempty"`
	Creator        []string       `json:"creator,omitempty"`
	VideoURL       string         `json:"video_url,omitempty"`
	ImageURL       string         `json:"image_url,omitempty"`
	PubDate        string         `json:"pubDate,omitempty"`
	PubDateTZ      string         `json:"pubDateTZ,omitempty"`
	SourceID       string         `json:"source_id,omitempty"`
	SourcePriority int            `json:"source_priority,omitempty"`
	SourceURL      string         `json:"source_url,omitempty"`
	SourceIcon     string         `json:"source_icon,omitempty"`
	SourceName     string         `json:"source_name,omitempty"`
	Language       string         `json:"language,omitempty"`
	Country        []string       `json:"country,omitempty"`
	Category       []string       `json:"category,omitempty"`
	AITag          []string       `json:"ai_tag,omitempty"`
	AIRegion       []string       `json:"ai_region,omitempty"`
	AIOrg          []string       `json:"ai_org,omitempty"`
	Sentiment      string         `json:"sentiment,omitempty"`
	SentimentStats map[string]any `json:"sentiment_stats,omitempty"`
	DataType       string         `json:"datatype,omitempty"`
	Symbol         []string       `json:"symbol,omitempty"`
	MarketID       []string       `json:"market_id,omitempty"`
}
