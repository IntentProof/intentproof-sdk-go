package intentproof

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSigningGoldenStripsSignatureBeforeCanonicalize(t *testing.T) {
	fixtureDir := filepath.Join("..", "testdata", "fixtures")
	unsignedRaw, err := os.ReadFile(filepath.Join(fixtureDir, "signing_unsigned_event.json"))
	if err != nil {
		t.Fatal(err)
	}
	unsigned, err := DecodeJSONMap(unsignedRaw)
	if err != nil {
		t.Fatal(err)
	}
	unsigned["signature"] = map[string]any{
		"alg":    "ed25519",
		"key_id": "inst_golden_test:k1",
		"value":  "AAAA",
	}

	expectedCanonical, err := os.ReadFile(filepath.Join(fixtureDir, "signing_canonical_utf8.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := CanonicalizeEvent(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(expectedCanonical) {
		t.Fatalf("canonical mismatch with signature present:\nwant %q\ngot  %q", expectedCanonical, got)
	}
}

func TestSigningGoldenHashStableAfterJSONDecode(t *testing.T) {
	fixtureDir := filepath.Join("..", "testdata", "fixtures")
	unsignedRaw, err := os.ReadFile(filepath.Join(fixtureDir, "signing_unsigned_event.json"))
	if err != nil {
		t.Fatal(err)
	}
	unsigned, err := DecodeJSONMap(unsignedRaw)
	if err != nil {
		t.Fatal(err)
	}
	expectedHash := strings.TrimSpace(readGoldenFile(t, filepath.Join(fixtureDir, "signing_event_hash.txt")))
	privateKeyB64 := strings.TrimSpace(readGoldenFile(t, filepath.Join(fixtureDir, "signing_private_key_b64.txt")))

	priv, err := LoadPrivateKey(privateKeyB64)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := SignEvent(unsigned, priv, "inst_golden_test")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := EventContentHash(signed)
	if err != nil {
		t.Fatal(err)
	}
	if hash != expectedHash {
		t.Fatalf("hash: want %s got %s", expectedHash, hash)
	}
	pub := priv.Public().(ed25519.PublicKey)
	ok, err := VerifyEventSignature(signed, pub)
	if err != nil || !ok {
		t.Fatalf("verify: ok=%v err=%v", ok, err)
	}
}

func readGoldenFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
