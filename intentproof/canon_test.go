package intentproof_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func TestPolicyBodyCrossCheck(t *testing.T) {
	body := map[string]any{
		"schema":         "intentproof.policy.v1",
		"policy_id":      "tnt.test",
		"policy_version": int64(1),
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
	got, err := intentproof.Canonicalize(body)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := os.ReadFile(filepath.Join("..", "testdata", "fixtures", "policy_body_canon.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimRight(string(wantBytes), "\r\n")
	if got != want {
		t.Fatalf("canonical mismatch:\nwant %q\ngot  %q", want, got)
	}
}
