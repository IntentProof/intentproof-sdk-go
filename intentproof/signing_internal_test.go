package intentproof

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestVerifyRejectsInvalidSignatureEncoding(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	event := map[string]any{
		"schema":    "intentproof.event.v1",
		"signature": map[string]any{"value": "!!!"},
	}
	ok, err := VerifyEventSignature(event, pub)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestVerifyRejectsMissingSignatureBlock(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	event := map[string]any{
		"schema":    "intentproof.event.v1",
		"signature": map[string]any{"value": 1},
	}
	ok, _ := VerifyEventSignature(event, pub)
	if ok {
		t.Fatal("expected false")
	}
}

func TestSignEventAndHashRoundTrip(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	event := map[string]any{
		"schema":   "intentproof.event.v1",
		"event_id": "evt_1",
	}
	signed, err := SignEvent(event, priv, "inst_test")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := EventContentHash(signed)
	if err != nil || hash == "" {
		t.Fatalf("hash=%s err=%v", hash, err)
	}
}

func TestEventContentHashRejectsUnmarshalableEvent(t *testing.T) {
	event := map[string]any{"bad": make(chan int)}
	if _, err := EventContentHash(event); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyEventSignaturePropagatesCanonicalizeError(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	event := map[string]any{
		"schema":    "intentproof.event.v1",
		"bad":       make(chan int),
		"signature": map[string]any{"alg": "ed25519", "value": "AAAA"},
	}
	ok, err := VerifyEventSignature(event, pub)
	if err == nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestSignEventRejectsUnmarshalableEvent(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	priv := ed25519.NewKeyFromSeed(seed)
	event := map[string]any{"bad": make(chan int)}
	if _, err := SignEvent(event, priv, "inst_test"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadPrivateKeyValid(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	priv := ed25519.NewKeyFromSeed(seed)
	raw := base64.StdEncoding.EncodeToString(seed)
	loaded, err := LoadPrivateKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Equal(priv) {
		t.Fatal("key mismatch")
	}
}
