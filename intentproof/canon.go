package intentproof

import (
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
