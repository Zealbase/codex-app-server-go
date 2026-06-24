// Package jsonx holds dependency-free helpers for extracting text and JSON
// candidates from loosely-typed payloads. These were factored out of the public
// codexgo package's output collector; they reference no protocol types and are
// independently testable.
package jsonx

import (
	"encoding/json"
	"strings"
)

// ExtractStructuredCandidate returns the first valid JSON document found within
// payload (including nested string-encoded JSON), or nil.
func ExtractStructuredCandidate(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	var data any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil
	}
	for _, candidate := range CollectJSONCandidates(data) {
		if json.Valid([]byte(candidate)) {
			return []byte(candidate)
		}
	}
	if json.Valid(payload) {
		return CloneBytes(payload)
	}
	return nil
}

// ExtractTextCandidate unmarshals payload and returns the best text value found.
func ExtractTextCandidate(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var data any
	if err := json.Unmarshal(payload, &data); err != nil {
		return ""
	}
	return ExtractTextValue(data)
}

// ExtractTextValue walks a decoded JSON value and returns the most relevant text
// (preferring text/content/message/output/value keys), or "".
func ExtractTextValue(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(StripCodeFence(x))
	case []any:
		for i := len(x) - 1; i >= 0; i-- {
			if text := ExtractTextValue(x[i]); text != "" {
				return text
			}
		}
	case map[string]any:
		for _, key := range []string{"text", "content", "message", "output", "value"} {
			if raw, ok := x[key]; ok {
				if text := ExtractTextValue(raw); text != "" {
					return text
				}
			}
		}
		for _, raw := range x {
			if text := ExtractTextValue(raw); text != "" {
				return text
			}
		}
	}
	return ""
}

// ExtractStringCandidate unmarshals payload and returns its best text value.
func ExtractStringCandidate(payload []byte) string {
	var data any
	if err := json.Unmarshal(payload, &data); err != nil {
		return ""
	}
	return ExtractTextValue(data)
}

// CollectJSONCandidates returns all valid JSON documents embedded in v (as
// strings or as marshaled objects), depth-first.
func CollectJSONCandidates(v any) []string {
	var out []string
	var walk func(any)
	walk = func(node any) {
		switch x := node.(type) {
		case string:
			s := strings.TrimSpace(StripCodeFence(x))
			if json.Valid([]byte(s)) {
				out = append(out, s)
			}
		case []any:
			for _, elem := range x {
				walk(elem)
			}
		case map[string]any:
			for _, elem := range x {
				walk(elem)
			}
			body, err := json.Marshal(x)
			if err == nil && json.Valid(body) {
				out = append(out, string(body))
			}
		}
	}
	walk(v)
	return out
}

// StripCodeFence removes a leading ```json/``` fence and trailing ``` from s.
func StripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// FirstNonEmpty returns the first value whose trimmed form is non-empty.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// CloneBytes returns a copy of data, or nil for empty input.
func CloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out
}
