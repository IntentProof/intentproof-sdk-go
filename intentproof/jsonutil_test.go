package intentproof

import "testing"

func TestDecodeJSONMapNormalizesWholeNumbers(t *testing.T) {
	m, err := DecodeJSONMap([]byte(`{"chain_position":1,"ratio":1.5}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["chain_position"].(int64); !ok {
		t.Fatalf("chain_position type: %T", m["chain_position"])
	}
	if _, ok := m["ratio"].(float64); !ok {
		t.Fatalf("ratio type: %T", m["ratio"])
	}
}
