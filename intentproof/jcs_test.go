package intentproof

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// portedVectors mirrors intentproof-spec/conformance/jcs_vectors.ts so that
// the Go implementation is verified against the same conformance set used
// by the TypeScript reference implementation.
var portedVectors = []struct {
	name     string
	inputJSON string
	expected string
}{
	{
		name:      "object key sort numbers",
		inputJSON: `{"b":2,"a":1}`,
		expected:  `{"a":1,"b":2}`,
	},
	{
		name:      "object key sort strings",
		inputJSON: `{"b":"2","a":"1"}`,
		expected:  `{"a":"1","b":"2"}`,
	},
	{
		name:      "nested empty container",
		inputJSON: `{"c":0,"b":[],"a":{}}`,
		expected:  `{"a":{},"b":[],"c":0}`,
	},
	{
		name:      "string keys lexicographic not numeric",
		inputJSON: `{"11":"eleven","10":"ten","1":"one"}`,
		expected:  `{"1":"one","10":"ten","11":"eleven"}`,
	},
	{
		name:      "float trims trailing zero",
		inputJSON: `{"b":1.2,"a":1.0}`,
		expected:  `{"a":1,"b":1.2}`,
	},
	{
		name:      "booleans",
		inputJSON: `{"b":true,"a":false}`,
		expected:  `{"a":false,"b":true}`,
	},
	{
		name:      "nulls",
		inputJSON: `{"b":null,"a":null}`,
		expected:  `{"a":null,"b":null}`,
	},
	{
		name:      "array order preserved",
		inputJSON: `{"b":[3,2,1],"a":[1,2,3]}`,
		expected:  `{"a":[1,2,3],"b":[3,2,1]}`,
	},
	{
		name:      "unicode literal",
		inputJSON: `{"unicode":"é","ascii":"e"}`,
		expected:  `{"ascii":"e","unicode":"é"}`,
	},
	{
		name:      "string escapes",
		inputJSON: `{"slash":"a/b","backslash":"a\\b"}`,
		expected:  `{"backslash":"a\\b","slash":"a/b"}`,
	},
}

func TestPortedVectorsFromSpec(t *testing.T) {
	for _, v := range portedVectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			got, err := marshalJCSRaw(json.RawMessage(v.inputJSON))
			if err != nil {
				t.Fatalf("MarshalRaw error: %v", err)
			}
			if string(got) != v.expected {
				t.Fatalf("\nexpected: %s\n     got: %s", v.expected, string(got))
			}
		})
	}
}

// Additional RFC 8785 conformance vectors covering corner cases not
// represented in the ported spec vectors.

func TestPrimitives(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  string
	}{
		{"null", nil, "null"},
		{"true", true, "true"},
		{"false", false, "false"},
		{"empty string", "", `""`},
		{"empty object", map[string]any{}, `{}`},
		{"empty array", []any{}, `[]`},
		{"ascii string", "hello", `"hello"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := marshalJCS(tc.input)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, string(got))
			}
		})
	}
}

func TestStringEscapes(t *testing.T) {
	// RFC 8785 §3.2.2.2 — minimal escapes only.
	cases := []struct {
		in   string
		want string
	}{
		{"\"", `"\""`},
		{"\\", `"\\"`},
		{"\b", `"\b"`},
		{"\f", `"\f"`},
		{"\n", `"\n"`},
		{"\r", `"\r"`},
		{"\t", `"\t"`},
		{"\x00", `"\u0000"`},
		{"\x01", `"\u0001"`},
		{"\x1f", `"\u001f"`},
		{"/", `"/"`},               // forward slash NOT escaped
		{"\x7f", "\"\x7f\""},       // DEL is NOT escaped (not C0 control)
		{"a\u0080b", "\"a\u0080b\""}, // C1 control literal
		{"héllo", `"héllo"`},
		{"\U0001F600", "\"\U0001F600\""}, // supplementary plane literal
	}
	for _, c := range cases {
		c := c
		t.Run(c.want, func(t *testing.T) {
			got, err := marshalJCS(c.in)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("expected %q, got %q", c.want, string(got))
			}
		})
	}
}

func TestNumberFormatting(t *testing.T) {
	// RFC 8785 §3.2.2.3 / ES6 Number.prototype.toString.
	cases := []struct {
		in   string
		want string
	}{
		{"0", "0"},
		{"-0", "0"},
		{"1", "1"},
		{"-1", "-1"},
		{"1.0", "1"},
		{"1.5", "1.5"},
		{"-1.5", "-1.5"},
		{"100", "100"},
		{"0.1", "0.1"},
		{"0.001", "0.001"},
		{"1e2", "100"},
		{"1e21", "1e+21"},
		{"1e20", "100000000000000000000"},
		{"1e-6", "0.000001"},
		{"1e-7", "1e-7"},
		{"1.5e-7", "1.5e-7"},
		// Large integer at the boundary of safe integers; still emitted
		// without scientific notation because n <= 21.
		{"9007199254740992", "9007199254740992"},
		// Just above 1e21 -> scientific.
		{"1e22", "1e+22"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in+"_to_"+c.want, func(t *testing.T) {
			got, err := marshalJCSRaw(json.RawMessage(c.in))
			if err != nil {
				t.Fatalf("MarshalRaw error: %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("expected %s, got %s", c.want, string(got))
			}
		})
	}
}

func TestRejectsNonFiniteNumbers(t *testing.T) {
	cases := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, f := range cases {
		if _, err := marshalJCS(f); err == nil {
			t.Fatalf("expected error for non-finite %v, got nil", f)
		}
	}
}

func TestUTF16KeyOrdering(t *testing.T) {
	// Keys containing characters in the supplementary plane sort
	// AFTER U+E000 (private use area) when compared by UTF-16 code
	// units, because supplementary characters are encoded as a high
	// surrogate in [D800, DBFF] which is less than U+E000... wait,
	// that means supplementary keys sort BEFORE U+E000 keys.
	//
	// Concretely: "\uE000" vs "\U0001F600" (= "\uD83D\uDE00").
	//   UTF-16(0xE000)      = [E000]
	//   UTF-16(0x1F600)     = [D83D, DE00]
	// 0xD83D < 0xE000, so the surrogate-pair key sorts first under
	// UTF-16 ordering. Under byte (UTF-8) ordering the opposite would
	// be true because the UTF-8 encoding of U+1F600 begins with 0xF0
	// which is greater than the UTF-8 of U+E000 starting at 0xEE.
	input := map[string]any{
		"\uE000":      1, // BMP, UTF-16 unit 0xE000
		"\U0001F600":  2, // supplementary, UTF-16 starts 0xD83D
	}
	got, err := marshalJCS(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	// We just check that the supplementary key appears first.
	supIdx := indexOf(string(got), "\U0001F600")
	bmpIdx := indexOf(string(got), "\uE000")
	if supIdx < 0 || bmpIdx < 0 {
		t.Fatalf("expected both keys present, got %s", string(got))
	}
	if supIdx >= bmpIdx {
		t.Fatalf("UTF-16 ordering violated:\n  %s\nsupIdx=%d bmpIdx=%d",
			string(got), supIdx, bmpIdx)
	}
}

// indexOf returns the index of substr in s, or -1 if not present.
func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}

func TestArrayElementOrderPreserved(t *testing.T) {
	got, err := marshalJCS([]any{3, 1, 2, "z", "a"})
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	want := `[3,1,2,"z","a"]`
	if string(got) != want {
		t.Fatalf("expected %s, got %s", want, string(got))
	}
}

func TestNestedObjectsSorted(t *testing.T) {
	got, err := marshalJCSRaw(json.RawMessage(`{"z":{"b":2,"a":1},"a":[{"c":3,"b":2}]}`))
	if err != nil {
		t.Fatalf("MarshalRaw error: %v", err)
	}
	want := `{"a":[{"b":2,"c":3}],"z":{"a":1,"b":2}}`
	if string(got) != want {
		t.Fatalf("expected %s, got %s", want, string(got))
	}
}

func TestRejectsTrailingTokens(t *testing.T) {
	if _, err := marshalJCSRaw(json.RawMessage(`{} {}`)); err == nil {
		t.Fatalf("expected error for trailing tokens")
	}
}

func TestRejectsTrailingNonEOF(t *testing.T) {
	// Malformed suffix after a valid JSON value must produce an error
	// (the decoder sees an invalid character, not just an extra token).
	if _, err := marshalJCSRaw(json.RawMessage(`{}x`)); err == nil {
		t.Fatalf("expected error for trailing non-EOF bytes")
	}
}

func TestRejectsDuplicateKeys(t *testing.T) {
	if _, err := marshalJCSRaw(json.RawMessage(`{"a":1,"a":2}`)); err == nil {
		t.Fatalf("expected error for duplicate object keys")
	}
}

func TestRejectsMalformedJSON(t *testing.T) {
	if _, err := marshalJCSRaw(json.RawMessage(`{`)); err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
}

// TestPortedPolicyFingerprintSetup verifies the same pre-fingerprint
// shape used by the spec's policy fingerprint test produces the
// expected SHA-256 hex digest under our canonicalizer. This is the
// cross-check against intentproof-spec/conformance/jcs_test.ts.
func TestPolicyFingerprintCrossCheck(t *testing.T) {
	// Same stub as jcs_test.ts after deleting the three excluded fields.
	stub := map[string]any{
		"schema":         "intentproof.policy.v1",
		"policy_id":      "tnt.test",
		"policy_version": 1,
		"tenant_id":      "tnt",
		"spec_version":   "1.0.0",
		"scope":          map[string]any{"any_event_action_in": []any{"a"}},
		"rules": []any{
			map[string]any{
				"id":       "r1",
				"category": "required",
				"severity": "high",
				"spec":     map[string]any{"action": "a"},
			},
		},
	}
	canonical, err := marshalJCS(stub)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	// We can't import crypto/sha256 conditionally; just compute and
	// check the hex matches the value embedded in the spec test.
	const wantHash = "7ffa54b2f15b9ab936a94eb3926a79bde8f66b0a81d0fee69b6c9d2c6a2fb07b"
	gotHash := sha256Hex(canonical)
	if gotHash != wantHash {
		t.Fatalf("policy fingerprint mismatch:\n  canonical = %s\n  want hash = %s\n   got hash = %s",
			string(canonical), wantHash, gotHash)
	}
}
