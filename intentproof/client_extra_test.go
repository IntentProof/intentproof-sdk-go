package intentproof_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func TestDefaultDataDir(t *testing.T) {
	dir := intentproof.DefaultDataDir()
	if dir == "" {
		t.Fatal("expected non-empty default data dir")
	}
}

func TestGettersBeforeConfigure(t *testing.T) {
	// Reset state by configuring then we need isolated package - instead test error paths
	// by using a fresh module is hard; test GetTenantID without configure returns default.
	if intentproof.GetTenantID() == "" {
		t.Fatal("expected default tenant")
	}
}

func TestConfigureUsesDefaultDataDirWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	os.Unsetenv("INTENTPROOF_OUTBOX_PATH")
	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		TenantID: "tnt_default_dir",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestConfigureWithEnvTenantAndOutboxPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INTENTPROOF_TENANT_ID", "tnt_from_env")
	t.Setenv("INTENTPROOF_OUTBOX_PATH", filepath.Join(dir, "custom.db"))
	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DataDir: filepath.Join(dir, "data"),
	}); err != nil {
		t.Fatal(err)
	}
	if intentproof.GetTenantID() != "tnt_from_env" {
		t.Fatalf("tenant: %s", intentproof.GetTenantID())
	}
}

func TestConfigureInvalidKeypairFails(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "keypair.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	dbPath, _ := testDirs(t)
	err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:   dbPath,
		DataDir:  dataDir,
		TenantID: "tnt_bad",
	})
	if err == nil {
		t.Fatal("expected configure error")
	}
}

func TestFlushWithoutExporter(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	intentproof.Flush() // no exporter configured
}

func TestPushSubjectMappingNoOp(t *testing.T) {
	intentproof.PushSubjectMapping("src", "type", "id")
}

func TestWrapGeneratesCorrelationIDWhenUnset(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_auto_corr")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x })
	fn(1)
	ob, _ := intentproof.GetOutbox()
	events, _ := ob.Events()
	if len(events) != 1 {
		t.Fatalf("events: %d", len(events))
	}
	cid, _ := events[0]["correlation_id"].(string)
	if cid == "" || cid == "corr-outer" {
		// should be req_* ULID, not empty or from other tests' fixed ids
		if len(cid) < 4 || cid[:4] != "req_" {
			t.Fatalf("correlation_id: %q", cid)
		}
	}
}
