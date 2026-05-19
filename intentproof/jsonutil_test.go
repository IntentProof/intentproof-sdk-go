package intentproof_test

import (
	"testing"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func TestDecodeJSONMapNormalizesIntegers(t *testing.T) {
	raw := []byte(`{"chain_position":1,"duration_ms":1,"inputs":[5]}`)
	m, err := intentproof.DecodeJSONMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["chain_position"].(int64); !ok {
		t.Fatalf("chain_position type: %T", m["chain_position"])
	}
}

func TestDecodeJSONMapKeepsFractionalNumbers(t *testing.T) {
	raw := []byte(`{"ratio":1.5}`)
	m, err := intentproof.DecodeJSONMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["ratio"].(float64); !ok {
		t.Fatalf("ratio type: %T", m["ratio"])
	}
}

func TestDecodeJSONMapRejectsNonObject(t *testing.T) {
	if _, err := intentproof.DecodeJSONMap([]byte(`[1,2,3]`)); err == nil {
		t.Fatal("expected error for JSON array")
	}
}
