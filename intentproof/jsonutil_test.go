package intentproof

import "testing"

func TestDecodeJSONMapRejectsNonObject(t *testing.T) {
	if _, err := DecodeJSONMap([]byte(`[1,2,3]`)); err == nil {
		t.Fatal("expected error for JSON array")
	}
}
