package intentproof

import (
	"encoding/json"
	"testing"
)

func TestNormalizeJSONNumberInvalidJSONNumber(t *testing.T) {
	v := normalizeJSONNumbers(json.Number("not-a-number"))
	if _, ok := v.(json.Number); !ok {
		t.Fatalf("type: %T", v)
	}
}

func TestNormalizeJSONNumberFromJSONNumberFloat(t *testing.T) {
	v := normalizeJSONNumbers(json.Number("1.5"))
	if _, ok := v.(float64); !ok {
		t.Fatalf("type: %T", v)
	}
}

func TestNormalizeJSONNumberFloat64Whole(t *testing.T) {
	v := normalizeJSONNumbers(float64(4))
	if _, ok := v.(int64); !ok {
		t.Fatalf("type: %T", v)
	}
}

func TestNormalizeJSONNumberFloat64(t *testing.T) {
	v := normalizeJSONNumbers(float64(1.5))
	if _, ok := v.(float64); !ok {
		t.Fatalf("type: %T", v)
	}
}

func TestNormalizeJSONNumberFallback(t *testing.T) {
	v := normalizeJSONNumbers(json.Number("not-a-number"))
	if _, ok := v.(json.Number); !ok {
		t.Fatalf("type: %T", v)
	}
}
