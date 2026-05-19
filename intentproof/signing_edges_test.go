package intentproof_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func TestVerifyRejectsMissingSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	event := map[string]any{"schema": "intentproof.event.v1", "event_id": "evt_unsigned"}
	ok, err := intentproof.VerifyEventSignature(event, priv.Public().(ed25519.PublicKey))
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	digest := make([]byte, 32)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, digest))
	event := map[string]any{
		"schema":    "intentproof.event.v1",
		"signature": map[string]any{"value": sig},
	}
	otherPub, _, _ := ed25519.GenerateKey(nil)
	ok, err := intentproof.VerifyEventSignature(event, otherPub)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestLoadPrivateKeyInvalidBase64(t *testing.T) {
	_, err := intentproof.LoadPrivateKey("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadPrivateKeyWrongLength(t *testing.T) {
	_, err := intentproof.LoadPrivateKey(base64.StdEncoding.EncodeToString([]byte{1, 2, 3}))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCanonicalizeErrorOnUnsupportedType(t *testing.T) {
	_, err := intentproof.Canonicalize(map[string]any{"bad": make(chan int)})
	if err == nil {
		t.Fatal("expected error")
	}
}
