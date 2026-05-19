package intentproof

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// SentinelPrevHash is the genesis prev_event_hash for chain position 1.
const SentinelPrevHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

// EventContentHash returns the sha256 content hash of a signed or unsigned event.
func EventContentHash(event map[string]any) (string, error) {
	canonical, err := canonicalizeEvent(event)
	if err != nil {
		return "", err
	}
	sum := sha256Sum([]byte(canonical))
	return "sha256:" + hex.EncodeToString(sum), nil
}

// CanonicalizeEvent returns JCS canonical JSON for an event without its signature.
func CanonicalizeEvent(event map[string]any) (string, error) {
	return canonicalizeEvent(event)
}

// SignEvent attaches an Ed25519 signature over the canonical event bytes.
func SignEvent(event map[string]any, privateKey ed25519.PrivateKey, instanceID string) (map[string]any, error) {
	canonical, err := canonicalizeEvent(event)
	if err != nil {
		return nil, err
	}
	digest := sha256Sum([]byte(canonical))
	sig := ed25519.Sign(privateKey, digest)
	signed := cloneMap(event)
	signed["signature"] = map[string]any{
		"alg":    "ed25519",
		"key_id": instanceID + ":k1",
		"value":  base64.StdEncoding.EncodeToString(sig),
	}
	return signed, nil
}

// VerifyEventSignature checks the Ed25519 signature on event.
func VerifyEventSignature(event map[string]any, publicKey ed25519.PublicKey) (bool, error) {
	sigBlock, ok := event["signature"].(map[string]any)
	if !ok {
		return false, nil
	}
	value, ok := sigBlock["value"].(string)
	if !ok {
		return false, nil
	}
	sig, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false, nil
	}
	canonical, err := canonicalizeEvent(event)
	if err != nil {
		return false, err
	}
	digest := sha256Sum([]byte(canonical))
	if !ed25519.Verify(publicKey, digest, sig) {
		return false, nil
	}
	return true, nil
}

// LoadPrivateKey decodes a base64-encoded 32-byte Ed25519 seed.
func LoadPrivateKey(rawB64 string) (ed25519.PrivateKey, error) {
	seed, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return nil, fmt.Errorf("intentproof: decode private key: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("intentproof: private key must be %d bytes", ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
