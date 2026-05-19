package intentproof

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// normalizeJSONNumbers converts JSON numbers to int64 when they are whole.
func normalizeJSONNumbers(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeJSONNumbers(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeJSONNumbers(val)
		}
		return out
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t
	case float64:
		if t == float64(int64(t)) {
			return int64(t)
		}
		return t
	default:
		return v
	}
}

// DecodeJSONMap decodes JSON objects with integer-friendly number types.
func DecodeJSONMap(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	norm := normalizeJSONNumbers(v)
	m, ok := norm.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("intentproof: decode JSON object: got %T", norm)
	}
	return m, nil
}
