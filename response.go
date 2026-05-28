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
// Use the Articles() / Aggregate() helpers to decode it into a typed shape.
type Response struct {
	Status          string          `json:"status,omitempty"`
	TotalResults    int             `json:"totalResults,omitempty"`
	Results         json.RawMessage `json:"results,omitempty"`
	NextPage        string          `json:"nextPage,omitempty"`
	ResponseHeaders http.Header     `json:"-"`
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

// Article is the typed shape of a single news result from the API. Fields use
// snake_case JSON tags to match the API; AI-enriched fields (ai_*) are set on
// articles where the platform produced them and are otherwise empty.
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
}
