package intentproof_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func TestLoadOrCreateKeypairExclusiveCreateRace(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	payload := map[string]string{
		"privateKey": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"instanceId": "inst_race2",
	}
	raw, _ := json.Marshal(payload)
	if err := os.WriteFile(keyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_race2",
	}); err != nil {
		t.Fatal(err)
	}
	id, _ := intentproof.GetInstanceID()
	if id != "inst_race2" {
		t.Fatalf("id %s", id)
	}
}

func TestLoadOrCreateKeypairRace(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	payload := map[string]string{
		"privateKey": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"instanceId": "inst_race",
	}
	raw, _ := json.Marshal(payload)

	// Simulate lost race: file exists before exclusive create completes.
	if err := os.WriteFile(keyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_race",
	}); err != nil {
		t.Fatal(err)
	}
	id, err := intentproof.GetInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	if id != "inst_race" {
		t.Fatalf("instance id: %s", id)
	}
}

func TestLoadRetriesUntilKeypairWritten(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	if err := os.WriteFile(keyPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	payload := map[string]string{
		"privateKey": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"instanceId": "inst_retry",
	}
	raw, _ := json.Marshal(payload)

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(keyPath, raw, 0o600)
	}()

	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_retry",
	}); err != nil {
		t.Fatal(err)
	}
	id, _ := intentproof.GetInstanceID()
	if id != "inst_retry" {
		t.Fatalf("instance id: %s", id)
	}
}
