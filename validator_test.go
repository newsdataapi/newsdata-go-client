package newsdataapi

import (
	"errors"
	"testing"
)

func TestArraysAreCommaJoined(t *testing.T) {
	got, err := validateAndEncode("latest", Params{"country": []string{"us", "gb"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("country") != "us,gb" {
		t.Errorf("country = %q, want %q", got.Get("country"), "us,gb")
	}
}

func TestBooleansAreCoercedToFlag(t *testing.T) {
	got, err := validateAndEncode("latest", Params{"full_content": true, "image": false})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("full_content") != "1" || got.Get("image") != "0" {
		t.Errorf("got %v", got)
	}
}

func TestKeysAreLowercased(t *testing.T) {
	got, err := validateAndEncode("latest", Params{"qInTitle": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("qintitle") != "hi" {
		t.Errorf("got %v", got)
	}
}

func TestNilValuesAreDropped(t *testing.T) {
	got, err := validateAndEncode("latest", Params{"q": "x", "country": nil})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("q") != "x" {
		t.Errorf("q missing: %v", got)
	}
	if _, present := got["country"]; present {
		t.Errorf("country should have been dropped: %v", got)
	}
}

func TestSizeUpperBoundRejected(t *testing.T) {
	_, err := validateAndEncode("latest", Params{"size": sizeMax + 1})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) || ve.Param != "size" {
		t.Errorf("expected NewsdataValidationError for size, got %v", err)
	}
}

func TestSizeWithinBoundsAccepted(t *testing.T) {
	got, err := validateAndEncode("latest", Params{"size": 50})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("size") != "50" {
		t.Errorf("got %v", got)
	}
}

func TestMutuallyExclusiveRejected(t *testing.T) {
	_, err := validateAndEncode("latest", Params{"q": "a", "qInTitle": "b"})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected NewsdataValidationError, got %v", err)
	}
}

func TestUnknownParameterRejected(t *testing.T) {
	_, err := validateAndEncode("latest", Params{"nope": "x"})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) || ve.Param != "nope" {
		t.Errorf("expected validation error for 'nope', got %v", err)
	}
}

func TestCryptoRejectsCountry(t *testing.T) {
	_, err := validateAndEncode("crypto", Params{"country": "us"})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected NewsdataValidationError, got %v", err)
	}
}

func TestSentimentScoreRequiresSentiment(t *testing.T) {
	_, err := validateAndEncode("latest", Params{"sentiment_score": 0.5})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) || ve.Param != "sentiment_score" {
		t.Errorf("expected validation error for sentiment_score, got %v", err)
	}
}

func TestSentimentScoreWithSentimentAccepted(t *testing.T) {
	got, err := validateAndEncode("latest", Params{
		"sentiment":       "positive",
		"sentiment_score": 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("sentiment") != "positive" || got.Get("sentiment_score") != "0.5" {
		t.Errorf("got %v", got)
	}
}

func TestCountRequiresDateRange(t *testing.T) {
	_, err := validateAndEncode("count", Params{"q": "x"})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestCountWithDatesAccepted(t *testing.T) {
	got, err := validateAndEncode("count", Params{
		"from_date": "2024-01-01",
		"to_date":   "2024-01-02",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("from_date") != "2024-01-01" || got.Get("to_date") != "2024-01-02" {
		t.Errorf("got %v", got)
	}
}

func TestRawQueryParsed(t *testing.T) {
	got, err := validateAndEncode("latest", Params{"rawQuery": "q=foo&country=us"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("q") != "foo" || got.Get("country") != "us" {
		t.Errorf("got %v", got)
	}
}

func TestRawQueryRejectsOtherParams(t *testing.T) {
	_, err := validateAndEncode("latest", Params{"rawQuery": "q=foo", "country": "us"})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestRawQueryRejectsUnknownKey(t *testing.T) {
	_, err := validateAndEncode("latest", Params{"rawQuery": "bogus=1"})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestRawQueryIgnoresEmbeddedAPIKey(t *testing.T) {
	got, err := validateAndEncode("latest", Params{"rawQuery": "apikey=secret&q=foo"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("q") != "foo" {
		t.Errorf("q missing: %v", got)
	}
	if got.Get("apikey") != "" {
		t.Errorf("apikey should be stripped: %v", got)
	}
}

func TestRawQueryAcceptsFullURL(t *testing.T) {
	got, err := validateAndEncode("latest", Params{
		"rawQuery": "https://newsdata.io/api/1/latest?q=foo&language=en",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("q") != "foo" || got.Get("language") != "en" {
		t.Errorf("got %v", got)
	}
}

func TestValidationErrorExposesParamName(t *testing.T) {
	_, err := validateAndEncode("latest", Params{"size": 999})
	var ve *NewsdataValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected NewsdataValidationError, got %v", err)
	}
	if ve.Param != "size" {
		t.Errorf("param = %q, want size", ve.Param)
	}
}
