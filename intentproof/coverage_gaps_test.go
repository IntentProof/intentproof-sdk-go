package intentproof

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoroutineIDEmptyIDField(t *testing.T) {
	if got := parseGoroutineIDFromStack([]byte("goroutine  [running]:\n")); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestHTTPExporterEnqueueMarshalFailure(t *testing.T) {
	exp := NewHTTPExporter("http://127.0.0.1:1/v1/events")
	exp.Enqueue(map[string]any{"bad": make(chan int)})
}

func TestHTTPExporterEnqueuePostFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	exp := NewHTTPExporter(srv.URL)
	exp.Enqueue(map[string]any{"schema": "intentproof.event.v1", "event_id": "evt_fail"})
	exp.Flush()
}

func TestLoadKeypairZeroAttempts(t *testing.T) {
	orig := loadKeypairAttempts
	loadKeypairAttempts = 0
	defer func() { loadKeypairAttempts = orig }()

	_, err := loadKeypair(filepath.Join(t.TempDir(), "keypair.json"))
	if err == nil {
		t.Fatal("expected load error")
	}
}

func TestLoadKeypairChmodBestEffort(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	if err := os.WriteFile(keyPath, []byte(`{"privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","instanceId":"inst_x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(keyPath, 0o400); err != nil {
		t.Skip("chmod not supported")
	}
	kp, err := loadKeypair(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if kp.InstanceID != "inst_x" {
		t.Fatalf("instance id: %q", kp.InstanceID)
	}
}

func TestEventsFailsWhenDBClosed(t *testing.T) {
	ob, err := OpenOutbox(filepath.Join(t.TempDir(), "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ob.db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ob.Events(); err == nil {
		t.Fatal("expected query error")
	}
}

func TestRecordChainedEventCommitFailure(t *testing.T) {
	ob, err := OpenOutbox(filepath.Join(t.TempDir(), "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()

	build := func(pos int, prev string) (map[string]any, string, error) {
		return map[string]any{"chain_position": pos}, "sha256:x", nil
	}
	if _, err := ob.RecordChainedEvent("c", "e1", build); err != nil {
		t.Fatal(err)
	}
	if err := ob.db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = ob.RecordChainedEvent("c", "e2", build)
	if err == nil {
		t.Fatal("expected commit error")
	}
}

func TestWrapFuncPanicAndRecordFailure(t *testing.T) {
	dir := t.TempDir()
	if err := Configure(ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_wf_panic_rec",
	}); err != nil {
		t.Fatal(err)
	}
	clientMu.Lock()
	instancePrivate = ed25519.PrivateKey{}
	clientMu.Unlock()

	fn := WrapFunc("Test", "test.action", func(_ struct{}) (int, error) {
		panic("boom")
	})
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		_, _ = fn(struct{}{})
	}()
	if !panicked {
		t.Fatal("expected panic")
	}
}
