package newsdataapi

import (
	"encoding/json"
	"strconv"
	"strings"
)

// planNoticePrefix starts the string the API sends in place of a field the
// plan does not include ("ONLY AVAILABLE IN PAID PLANS", "ONLY AVAILABLE IN
// PROFESSIONAL AND CORPORATE PLANS", "ONLY AVAILABLE IN CORPORATE PLANS").
const planNoticePrefix = "ONLY AVAILABLE IN"

// IsPlanNotice reports whether a value is the API's "not in your plan" notice
// rather than real data. The API sends that notice in the field itself, so a
// withheld list field arrives as one notice string and a withheld sentiment
// carries it in Label:
//
//	if len(a.Country) == 1 && newsdataapi.IsPlanNotice(a.Country[0]) {
//	    // the plan does not include country tagging
//	}
func IsPlanNotice(value string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(value)), planNoticePrefix)
}

// StringList is a list of strings that also decodes from a single JSON string.
//
// Most v2 list fields (Country, Keyword, AI.Tag, ...) are arrays, but the API
// replaces the whole field with one "not in your plan" notice string when the
// plan does not include it. Decoding into []string would fail on that; this
// type yields a one-element list instead, which IsPlanNotice recognises.
type StringList []string

// UnmarshalJSON accepts a JSON array of strings, a single JSON string, or null.
func (s *StringList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		*s = nil
		return nil
	}

	if trimmed[0] == '[' {
		var items []string
		if err := json.Unmarshal(data, &items); err != nil {
			return err
		}
		*s = items
		return nil
	}

	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*s = StringList{single}
	return nil
}

// ArticleV2 is the typed shape of a single article from API v2.
//
// v2 renamed and regrouped the v1 fields: article_id became id, link became
// url, pubDate became published_at, description became meta_description, and
// the source, media and AI fields moved into the Source, Media and AI groups.
// Use ArticleV2 for the v2 endpoints and the real-time stream; use Article for
// the v1 endpoints.
//
// Field names are the API's own paths, the same ones ExcludeField accepts and
// the same ones the NewsData documentation uses, so one vocabulary covers
// parameters, results and docs.
//
// A field the plan does not include holds a notice string instead of data; see
// IsPlanNotice.
type ArticleV2 struct {
	ID              string          `json:"id"`
	Title           string          `json:"title,omitempty"`
	URL             string          `json:"url,omitempty"`
	MetaDescription string          `json:"meta_description,omitempty"`
	Content         string          `json:"content,omitempty"`
	PublishedAt     string          `json:"published_at,omitempty"`
	FetchedAt       string          `json:"fetched_at,omitempty"`
	Language        string          `json:"language,omitempty"`
	DataType        string          `json:"datatype,omitempty"`
	Duplicate       bool            `json:"duplicate,omitempty"`
	Country         StringList      `json:"country,omitempty"`
	Category        StringList      `json:"category,omitempty"`
	Keyword         StringList      `json:"keyword,omitempty"`
	Creator         StringList      `json:"creator,omitempty"`
	Coin            StringList      `json:"coin,omitempty"`
	Ticker          StringList      `json:"ticker,omitempty"`
	MarketID        StringList      `json:"market_id,omitempty"`
	Source          ArticleV2Source `json:"source,omitempty"`
	Media           ArticleV2Media  `json:"media,omitempty"`
	AI              ArticleV2AI     `json:"ai,omitempty"`
}

// ArticleV2Source is the publisher an article came from (the v2 "source"
// group). It replaces v1's source_id / source_name / source_url /
// source_icon / source_priority fields.
type ArticleV2Source struct {
	Name     string      `json:"name,omitempty"`
	URL      string      `json:"url,omitempty"`
	IconURL  string      `json:"icon_url,omitempty"`
	Priority FlexibleInt `json:"priority,omitempty"`
}

// ArticleV2Media holds an article's media addresses (the v2 "media" group). It
// replaces v1's image_url / video_url fields.
type ArticleV2Media struct {
	ImageURL string `json:"image_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
}

// ArticleV2AI holds the platform's AI enrichment (the v2 "ai" group). It
// replaces v1's sentiment / sentiment_stats / ai_tag / ai_region / ai_org
// fields and adds Summary. Fields are empty on articles the platform did not
// enrich, and hold a notice when the plan does not include them.
type ArticleV2AI struct {
	Summary      string             `json:"summary,omitempty"`
	Sentiment    ArticleV2Sentiment `json:"sentiment,omitempty"`
	Tag          StringList         `json:"tag,omitempty"`
	Region       StringList         `json:"region,omitempty"`
	Organization StringList         `json:"organization,omitempty"`
}

// ArticleV2Sentiment is an article's sentiment classification: a Label
// ("positive", "neutral", "negative") and the per-class confidence Score.
//
// Score is a map rather than three fields so a class the platform adds later
// is still decoded; the keys today are "negative", "neutral" and "positive".
type ArticleV2Sentiment struct {
	Label string             `json:"label,omitempty"`
	Score map[string]float64 `json:"score,omitempty"`
}

// UnmarshalJSON accepts the sentiment object or, when the plan does not
// include sentiment, the bare notice string the API sends in its place (which
// lands in Label, where IsPlanNotice recognises it).
func (s *ArticleV2Sentiment) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		*s = ArticleV2Sentiment{}
		return nil
	}

	if trimmed[0] == '"' {
		var notice string
		if err := json.Unmarshal(data, &notice); err != nil {
			return err
		}
		*s = ArticleV2Sentiment{Label: notice}
		return nil
	}

	var raw struct {
		Label string             `json:"label"`
		Score map[string]float64 `json:"score"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Label = raw.Label
	s.Score = raw.Score
	return nil
}

// FlexibleInt is an int that also decodes from a JSON string, so a numeric
// field the API quotes ("1520") still reaches callers as a number.
type FlexibleInt int

// UnmarshalJSON accepts a JSON number, a JSON string holding a number, or null.
func (f *FlexibleInt) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		*f = 0
		return nil
	}

	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			*f = 0
			return nil
		}
		n, err := strconv.Atoi(text)
		if err != nil {
			// A non-numeric string (a plan notice, for instance) is not a
			// priority; report no priority rather than failing the article.
			*f = 0
			return nil
		}
		*f = FlexibleInt(n)
		return nil
	}

	var n float64
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = FlexibleInt(int(n))
	return nil
}

// Int returns the value as a plain int.
func (f FlexibleInt) Int() int { return int(f) }
