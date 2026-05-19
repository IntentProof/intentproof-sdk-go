package intentproof

import (
	"encoding/json"
	"fmt"

	"github.com/intentproof/intentproof-sdk-go/internal/canon"
)

// Canonicalize returns the RFC 8785 canonical JSON encoding of v.
func Canonicalize(v any) (string, error) {
	b, err := canon.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// canonicalizeEvent returns canonical JSON for an unsigned execution event.
func canonicalizeEvent(event map[string]any) (string, error) {
	unsigned := make(map[string]any, len(event))
	for k, v := range event {
		if k == "signature" {
			continue
		}
		unsigned[k] = v
	}
	return Canonicalize(unsigned)
}

// decodeJSONStringValue canonicalizes a value that may already be JSON text.
func decodeJSONStringValue(s string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s, nil
	}
	return v, nil
}

// canonicalizeDecoded is used when inputs are JSON strings per JCS rules.
func canonicalizeDecoded(v any) (string, error) {
	switch t := v.(type) {
	case string:
		decoded, err := decodeJSONStringValue(t)
		if err != nil {
			return "", err
		}
		if decoded == t {
			return Canonicalize(t)
		}
		return Canonicalize(decoded)
	default:
		return Canonicalize(v)
	}
}

// mustCanonicalize panics only in tests; production paths return errors.
func mustCanonicalize(v any) string {
	s, err := Canonicalize(v)
	if err != nil {
		panic(fmt.Sprintf("intentproof: canonicalize: %v", err))
	}
	return s
}
