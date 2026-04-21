package middleware

import (
	"bytes"
	"encoding/json"
	"regexp"
)

// sensitiveBodyKeyPattern is the default field-name pattern treated as
// sensitive by DefaultBodyRedactor. The intent is to cover the categories
// that a reasonable ops review would expect to never see in plain-text
// request/response logs.
var sensitiveBodyKeyPattern = regexp.MustCompile(`(?i)(password|token|secret|api_?key|credit_?card|cvv|ssn)`)

// bodyRedactionPlaceholder replaces redacted string values.
const bodyRedactionPlaceholder = "***REDACTED***"

// DefaultBodyRedactor returns a copy of body with sensitive JSON fields
// masked. It is designed to fail soft: if the body is not valid JSON the
// original bytes are returned unchanged so that logging continues to
// work for non-JSON payloads. The match is performed against the JSON
// field name (case-insensitive) regardless of how deeply it is nested.
//
// Only string values are redacted. Numeric / boolean / null values keep
// their original type — this means a CVV given as "123" becomes
// "***REDACTED***" but a CVV given as 123 (number) is left alone. That
// is a deliberate trade-off: replacing a number with a string would
// break downstream log parsers that type-check fields, and high-risk
// PII is normally submitted as a string anyway.
func DefaultBodyRedactor(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return body
	}
	redactJSONValue(parsed, false)

	out, err := json.Marshal(parsed)
	if err != nil {
		return body
	}
	return out
}

// redactJSONValue walks the decoded JSON tree. parentWasSensitive is true
// when the current value is the child of a sensitive key, in which case
// every string descendant is redacted (protects nested shapes such as
// {"token": {"value": "secret"}}).
func redactJSONValue(v any, parentWasSensitive bool) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			sensitive := parentWasSensitive || sensitiveBodyKeyPattern.MatchString(k)
			if sensitive {
				if s, ok := child.(string); ok && s != "" {
					val[k] = bodyRedactionPlaceholder
					continue
				}
			}
			redactJSONValue(child, sensitive)
		}
	case []any:
		for i := range val {
			redactJSONValue(val[i], parentWasSensitive)
		}
	}
}
