package newsdataapi

import (
	"encoding/json"
	"testing"
)

// v2Article is one article exactly as API v2 sends it, with every group
// populated. The real-time stream sends this same layout.
const v2Article = `{
  "id": "9f8e7d6c5b4a39281706f5e4d3c2b1a0",
  "title": "Central bank holds rates steady as inflation cools",
  "meta_description": "The central bank kept its key rate unchanged on Wednesday.",
  "content": "Full article text...",
  "url": "https://www.example.com/business/central-bank-holds-rates",
  "published_at": "2026-10-22T06:15:00Z",
  "fetched_at": "2026-10-22T06:18:42Z",
  "datatype": "news",
  "language": "english",
  "country": ["india"],
  "category": ["business"],
  "keyword": ["interest rates", "inflation"],
  "creator": ["Jane Doe"],
  "duplicate": false,
  "media": {"image_url": "https://www.example.com/images/rates.jpg", "video_url": null},
  "source": {
    "name": "Example News",
    "url": "https://www.example.com",
    "icon_url": "https://www.example.com/icon.png",
    "priority": 1520
  },
  "ai": {
    "summary": "The central bank left its policy rate unchanged.",
    "sentiment": {"label": "neutral", "score": {"negative": 4.12, "neutral": 90.33, "positive": 5.55}},
    "tag": ["banking and finance", "economy"],
    "region": ["mumbai,maharashtra,india", "delhi,india"],
    "organization": ["reserve bank of india"]
  }
}`

func TestArticleV2DecodesEveryField(t *testing.T) {
	var a ArticleV2
	if err := json.Unmarshal([]byte(v2Article), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if a.ID != "9f8e7d6c5b4a39281706f5e4d3c2b1a0" {
		t.Errorf("ID = %q", a.ID)
	}
	if a.URL != "https://www.example.com/business/central-bank-holds-rates" {
		t.Errorf("URL = %q", a.URL)
	}
	if a.PublishedAt != "2026-10-22T06:15:00Z" {
		t.Errorf("PublishedAt = %q", a.PublishedAt)
	}
	if a.FetchedAt != "2026-10-22T06:18:42Z" {
		t.Errorf("FetchedAt = %q", a.FetchedAt)
	}
	if a.MetaDescription != "The central bank kept its key rate unchanged on Wednesday." {
		t.Errorf("MetaDescription = %q", a.MetaDescription)
	}
	if a.Language != "english" || a.DataType != "news" {
		t.Errorf("Language = %q, DataType = %q", a.Language, a.DataType)
	}
	if a.Duplicate {
		t.Error("Duplicate should be false")
	}
	if len(a.Keyword) != 2 || a.Keyword[0] != "interest rates" {
		t.Errorf("Keyword = %v", a.Keyword)
	}
	if len(a.Country) != 1 || a.Country[0] != "india" {
		t.Errorf("Country = %v", a.Country)
	}
}

func TestArticleV2DecodesGroupedFields(t *testing.T) {
	var a ArticleV2
	if err := json.Unmarshal([]byte(v2Article), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if a.Source.Name != "Example News" {
		t.Errorf("Source.Name = %q", a.Source.Name)
	}
	if a.Source.URL != "https://www.example.com" {
		t.Errorf("Source.URL = %q", a.Source.URL)
	}
	if a.Source.IconURL != "https://www.example.com/icon.png" {
		t.Errorf("Source.IconURL = %q", a.Source.IconURL)
	}
	if a.Source.Priority.Int() != 1520 {
		t.Errorf("Source.Priority = %d", a.Source.Priority.Int())
	}
	if a.Media.ImageURL != "https://www.example.com/images/rates.jpg" {
		t.Errorf("Media.ImageURL = %q", a.Media.ImageURL)
	}
	if a.Media.VideoURL != "" {
		t.Errorf("Media.VideoURL = %q, want empty for a null", a.Media.VideoURL)
	}
	if a.AI.Summary != "The central bank left its policy rate unchanged." {
		t.Errorf("AI.Summary = %q", a.AI.Summary)
	}
	if a.AI.Sentiment.Label != "neutral" {
		t.Errorf("AI.Sentiment.Label = %q", a.AI.Sentiment.Label)
	}
	if got := a.AI.Sentiment.Score["neutral"]; got != 90.33 {
		t.Errorf(`Score["neutral"] = %v`, got)
	}
	if len(a.AI.Tag) != 2 || a.AI.Tag[1] != "economy" {
		t.Errorf("AI.Tag = %v", a.AI.Tag)
	}
	// Region entries contain commas themselves and must not be split.
	if len(a.AI.Region) != 2 || a.AI.Region[0] != "mumbai,maharashtra,india" {
		t.Errorf("AI.Region = %v", a.AI.Region)
	}
	if len(a.AI.Organization) != 1 || a.AI.Organization[0] != "reserve bank of india" {
		t.Errorf("AI.Organization = %v", a.AI.Organization)
	}
}

func TestArticleV2MissingGroupsAreZero(t *testing.T) {
	var a ArticleV2
	if err := json.Unmarshal([]byte(`{"id":"x","title":"hi"}`), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Source.Name != "" || a.Media.ImageURL != "" || a.AI.Summary != "" {
		t.Error("absent groups should leave their fields empty")
	}
	if a.AI.Sentiment.Label != "" || a.AI.Sentiment.Score != nil {
		t.Error("absent sentiment should be the zero value")
	}
	if a.Country != nil {
		t.Errorf("Country = %v, want nil", a.Country)
	}
}

// A field the plan does not include arrives as a notice string where an array
// is documented. Decoding must not fail on it.
func TestArticleV2PlanNoticeInAListField(t *testing.T) {
	var a ArticleV2
	body := `{"id":"x","ai":{"tag":"ONLY AVAILABLE IN PROFESSIONAL AND CORPORATE PLANS"}}`
	if err := json.Unmarshal([]byte(body), &a); err != nil {
		t.Fatalf("a plan notice must not fail decoding: %v", err)
	}
	if len(a.AI.Tag) != 1 {
		t.Fatalf("AI.Tag = %v, want one notice entry", a.AI.Tag)
	}
	if !IsPlanNotice(a.AI.Tag[0]) {
		t.Errorf("IsPlanNotice(%q) = false", a.AI.Tag[0])
	}
}

func TestArticleV2PlanNoticeInSentiment(t *testing.T) {
	var a ArticleV2
	// The notice can replace the whole object or sit in its label.
	for _, body := range []string{
		`{"id":"x","ai":{"sentiment":"ONLY AVAILABLE IN PAID PLANS"}}`,
		`{"id":"x","ai":{"sentiment":{"label":"ONLY AVAILABLE IN PAID PLANS"}}}`,
	} {
		if err := json.Unmarshal([]byte(body), &a); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if !IsPlanNotice(a.AI.Sentiment.Label) {
			t.Errorf("IsPlanNotice(%q) = false", a.AI.Sentiment.Label)
		}
	}
}

func TestIsPlanNotice(t *testing.T) {
	for _, s := range []string{
		"ONLY AVAILABLE IN PAID PLANS",
		"ONLY AVAILABLE IN PROFESSIONAL AND CORPORATE PLANS",
		"  only available in corporate plans  ",
	} {
		if !IsPlanNotice(s) {
			t.Errorf("IsPlanNotice(%q) = false", s)
		}
	}
	for _, s := range []string{"", "india", "neutral", "available in paid plans"} {
		if IsPlanNotice(s) {
			t.Errorf("IsPlanNotice(%q) = true", s)
		}
	}
}

func TestFlexibleIntAcceptsNumberStringAndNull(t *testing.T) {
	var s ArticleV2Source
	if err := json.Unmarshal([]byte(`{"priority":1520}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Priority.Int() != 1520 {
		t.Errorf("number: %d", s.Priority.Int())
	}

	if err := json.Unmarshal([]byte(`{"priority":"1520"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Priority.Int() != 1520 {
		t.Errorf("quoted number: %d", s.Priority.Int())
	}

	if err := json.Unmarshal([]byte(`{"priority":null}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Priority.Int() != 0 {
		t.Errorf("null: %d", s.Priority.Int())
	}

	// A non-numeric string is not a priority, but must not fail the article.
	if err := json.Unmarshal([]byte(`{"priority":"ONLY AVAILABLE IN PAID PLANS"}`), &s); err != nil {
		t.Fatalf("non-numeric priority must not fail decoding: %v", err)
	}
	if s.Priority.Int() != 0 {
		t.Errorf("notice: %d, want 0", s.Priority.Int())
	}
}

func TestResponseArticlesV2(t *testing.T) {
	var resp Response
	body := `{"status":"success","total_results":2841,"next_page":"1792649700000000000","results":[` + v2Article + `]}`
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TotalResults != 2841 || resp.NextPage != "1792649700000000000" {
		t.Errorf("envelope: total=%d next=%q", resp.TotalResults, resp.NextPage)
	}

	articles, err := resp.ArticlesV2()
	if err != nil {
		t.Fatalf("ArticlesV2: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles", len(articles))
	}
	if articles[0].Source.Name != "Example News" {
		t.Errorf("Source.Name = %q", articles[0].Source.Name)
	}
}

func TestResponseArticlesV2OnAnAggregateIsEmpty(t *testing.T) {
	var resp Response
	if err := json.Unmarshal([]byte(`{"status":"success","results":{"2026-10-22":5}}`), &resp); err != nil {
		t.Fatal(err)
	}
	articles, err := resp.ArticlesV2()
	if err != nil {
		t.Fatalf("ArticlesV2: %v", err)
	}
	if articles != nil {
		t.Errorf("got %v, want nil for an aggregate page", articles)
	}
}
