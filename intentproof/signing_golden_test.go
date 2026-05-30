package intentproof_test

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func signingFixtureDir(t *testing.T) string {
	t.Helper()
	if specDir := strings.TrimSpace(os.Getenv("INTENTPROOF_SPEC_DIR")); specDir != "" {
		if !filepath.IsAbs(specDir) {
			specDir = filepath.Join("..", specDir)
		}
		return filepath.Join(specDir, "golden", "sdk-signing")
	}
	return filepath.Join("..", "testdata", "fixtures")
}

func TestSigningGoldenBytes(t *testing.T) {
	fixtureDir := signingFixtureDir(t)
	unsignedRaw, err := os.ReadFile(filepath.Join(fixtureDir, "signing_unsigned_event.json"))
	if err != nil {
		t.Fatal(err)
	}
	unsigned, err := intentproof.DecodeJSONMap(unsignedRaw)
	if err != nil {
		t.Fatal(err)
	}

	expectedCanonical, err := os.ReadFile(filepath.Join(fixtureDir, "signing_canonical_utf8.txt"))
	if err != nil {
		t.Fatal(err)
	}
	expectedHash := strings.TrimSpace(readFile(t, filepath.Join(fixtureDir, "signing_event_hash.txt")))
	expectedSig := strings.TrimSpace(readFile(t, filepath.Join(fixtureDir, "signing_signature_b64.txt")))
	privateKeyB64 := strings.TrimSpace(readFile(t, filepath.Join(fixtureDir, "signing_private_key_b64.txt")))

	gotCanonical, err := intentproof.CanonicalizeEvent(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	if gotCanonical != string(expectedCanonical) {
		t.Fatalf("canonical mismatch:\nwant %q\ngot  %q", expectedCanonical, gotCanonical)
	}

	priv, err := intentproof.LoadPrivateKey(privateKeyB64)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := intentproof.SignEvent(unsigned, priv, "inst_golden_test")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := intentproof.EventContentHash(signed)
	if err != nil {
		t.Fatal(err)
	}
	if hash != expectedHash {
		t.Fatalf("hash: want %s got %s", expectedHash, hash)
	}
	sigBlock, _ := signed["signature"].(map[string]any)
	if sigBlock["value"] != expectedSig {
		t.Fatalf("signature: want %s got %v", expectedSig, sigBlock["value"])
	}
	pub := priv.Public().(ed25519.PublicKey)
	ok, err := intentproof.VerifyEventSignature(signed, pub)
	if err != nil || !ok {
		t.Fatalf("verify: ok=%v err=%v", ok, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
