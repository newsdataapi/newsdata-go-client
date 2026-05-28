package newsdataapi

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Params is the request payload for any endpoint method. Keys are case-
// insensitive (the API normalises to lowercase, and so do we) and values may be
// a string, a []string (comma-joined), a bool, an int, a float, or *bool /
// *float64 if you need to distinguish "unset" from a zero value.
//
// A few keys are interpreted specially and never sent to the API:
//
//	"rawQuery" (string)       — sent verbatim; mutually exclusive with all other
//	                            params; parsed and validated against the
//	                            endpoint's allowed keys.
//
// See the README for the per-endpoint accepted parameter list.
type Params map[string]any

// validateAndEncode mirrors the Python/PHP/Node client validator: it lowercases
// keys, drops nil/empty values, enforces mutually-exclusive groups, the
// sentiment_score-requires-sentiment rule, the from_date/to_date requirement on
// count endpoints, and the size 1..50 bound. Lists are comma-joined; booleans
// become "1"/"0".
//
// Returns url-encoded values ready to be appended to the request URL.
func validateAndEncode(endpoint string, params Params) (url.Values, error) {
	allowed, ok := filters[endpoint]
	if !ok {
		return nil, &NewsdataValidationError{Message: "unknown endpoint: " + endpoint}
	}

	// Lowercase keys; drop nil values.
	lowered := make(map[string]any, len(params))
	for k, v := range params {
		if v == nil {
			continue
		}
		lowered[strings.ToLower(k)] = v
	}

	// rawQuery: mutually exclusive with every other parameter.
	if raw, ok := lowered["rawquery"]; ok {
		others := make([]string, 0, len(lowered)-1)
		for k := range lowered {
			if k != "rawquery" {
				others = append(others, k)
			}
		}
		if len(others) > 0 {
			sort.Strings(others)
			return nil, &NewsdataValidationError{
				Param:   "rawQuery",
				Message: fmt.Sprintf("rawQuery cannot be combined with other parameters; got rawQuery and %v", others),
			}
		}
		rawStr, ok := raw.(string)
		if !ok {
			return nil, &NewsdataValidationError{Param: "rawQuery", Message: "rawQuery must be a string"}
		}
		return parseRawQuery(rawStr, allowed)
	}

	// Count endpoints require an explicit date range.
	if requiresDateRange[endpoint] {
		for _, required := range []string{"from_date", "to_date"} {
			if v, ok := lowered[required]; !ok || isEmptyString(v) {
				return nil, &NewsdataValidationError{
					Param:   required,
					Message: required + " is required for the " + endpoint + " endpoint",
				}
			}
		}
	}

	// Mutually-exclusive groups.
	for _, group := range mutexGroups {
		set := []string{}
		for _, name := range group {
			if _, ok := lowered[name]; ok {
				set = append(set, name)
			}
		}
		if len(set) > 1 {
			return nil, &NewsdataValidationError{
				Param:   set[0],
				Message: fmt.Sprintf("these parameters are mutually exclusive: %v", set),
			}
		}
	}

	// sentiment_score requires sentiment.
	if _, hasScore := lowered["sentiment_score"]; hasScore {
		if _, hasSentiment := lowered["sentiment"]; !hasSentiment {
			return nil, &NewsdataValidationError{
				Param:   "sentiment_score",
				Message: "sentiment_score requires sentiment to be set",
			}
		}
	}

	// Per-param validation + coercion.
	out := url.Values{}
	for name, value := range lowered {
		if !allowed[name] {
			return nil, &NewsdataValidationError{
				Param:   name,
				Message: "unsupported parameter for the " + endpoint + " endpoint",
			}
		}
		encoded, err := coerce(name, value)
		if err != nil {
			return nil, err
		}
		out.Set(name, encoded)
	}
	return out, nil
}

func coerce(name string, value any) (string, error) {
	switch {
	case boolParams[name]:
		return coerceBool(name, value)
	case intParams[name]:
		return coerceInt(name, value)
	case floatParams[name]:
		return coerceFloat(name, value)
	default:
		return coerceString(name, value)
	}
}

func coerceBool(name string, value any) (string, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	case *bool:
		if v == nil {
			return "", &NewsdataValidationError{Param: name, Message: "must be a boolean"}
		}
		if *v {
			return "1", nil
		}
		return "0", nil
	case int:
		if v == 0 {
			return "0", nil
		}
		if v == 1 {
			return "1", nil
		}
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes":
			return "1", nil
		case "0", "false", "no":
			return "0", nil
		}
	}
	return "", &NewsdataValidationError{Param: name, Message: "must be a boolean"}
}

func coerceInt(name string, value any) (string, error) {
	var n int
	switch v := value.(type) {
	case int:
		n = v
	case int32:
		n = int(v)
	case int64:
		n = int(v)
	case string:
		x, err := strconv.Atoi(v)
		if err != nil {
			return "", &NewsdataValidationError{Param: name, Message: name + " must be an integer"}
		}
		n = x
	default:
		return "", &NewsdataValidationError{Param: name, Message: name + " must be an integer"}
	}
	if name == "size" && (n < sizeMin || n > sizeMax) {
		return "", &NewsdataValidationError{
			Param:   "size",
			Message: fmt.Sprintf("size must be between %d and %d (got %d)", sizeMin, sizeMax, n),
		}
	}
	return strconv.Itoa(n), nil
}

func coerceFloat(name string, value any) (string, error) {
	switch v := value.(type) {
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case *float64:
		if v == nil {
			return "", &NewsdataValidationError{Param: name, Message: name + " must be a number"}
		}
		return strconv.FormatFloat(*v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	case string:
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", &NewsdataValidationError{Param: name, Message: name + " must be a number"}
		}
		return v, nil
	}
	return "", &NewsdataValidationError{Param: name, Message: name + " must be a number"}
}

func coerceString(name string, value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []string:
		return strings.Join(v, ","), nil
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return "", &NewsdataValidationError{Param: name, Message: "all items in " + name + " must be strings"}
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ","), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	}
	return "", &NewsdataValidationError{
		Param:   name,
		Message: name + " must be a string or []string",
	}
}

// parseRawQuery parses a query string fragment or full URL into a validated
// url.Values, rejecting unknown keys for the endpoint and stripping any
// embedded apikey.
func parseRawQuery(raw string, allowed map[string]bool) (url.Values, error) {
	if raw == "" {
		return nil, &NewsdataValidationError{Param: "rawQuery", Message: "rawQuery must be a non-empty string"}
	}

	queryString := raw
	// Full URL? Extract just the query part.
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		queryString = u.RawQuery
	}
	queryString = strings.TrimPrefix(queryString, "?")

	values, err := url.ParseQuery(queryString)
	if err != nil {
		return nil, &NewsdataValidationError{Param: "rawQuery", Message: "invalid rawQuery: " + err.Error()}
	}

	out := url.Values{}
	for k, vs := range values {
		name := strings.ToLower(strings.TrimSpace(k))
		if name == "" {
			continue
		}
		if name == "apikey" { // supplied by the client
			continue
		}
		if !allowed[name] {
			return nil, &NewsdataValidationError{Param: k, Message: "unknown parameter in rawQuery: " + k}
		}
		if len(vs) == 0 || vs[0] == "" {
			return nil, &NewsdataValidationError{Param: k, Message: "parameter " + k + " in rawQuery must have a value"}
		}
		out.Set(name, vs[0])
	}
	return out, nil
}

func isEmptyString(v any) bool {
	s, ok := v.(string)
	return ok && s == ""
}
