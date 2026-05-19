package intentproof

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestWrapPanicRecordsEvent(t *testing.T) {
	dir := t.TempDir()
	if err := Configure(ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_panic",
	}); err != nil {
		t.Fatal(err)
	}
	fn := Wrap("Test", "test.action", func(_ struct{}) int {
		panic("boom")
	})
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		RunWithCorrelationID("corr-wrap-panic", func() { fn(struct{}{}) })
	}()
	if !panicked {
		t.Fatal("expected panic")
	}

	ob, err := GetOutbox()
	if err != nil {
		t.Fatal(err)
	}
	events, err := ob.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[len(events)-1]["status"] != "error" {
		t.Fatalf("events: %+v", events)
	}
}

func TestUntrustedPayloadBranches(t *testing.T) {
	if untrustedPayload(nil, nil, "ok") {
		t.Fatal("expected false without inputs or output")
	}
	if !untrustedPayload([]any{1}, nil, "ok") {
		t.Fatal("expected true with inputs")
	}
	if !untrustedPayload(nil, map[string]any{"x": 1}, "ok") {
		t.Fatal("expected true with output")
	}
	if untrustedPayload(nil, nil, "error") {
		t.Fatal("expected false on error without inputs")
	}
}

func TestWrapWithInputsMarksUntrusted(t *testing.T) {
	dir := t.TempDir()
	if err := Configure(ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_inputs",
	}); err != nil {
		t.Fatal(err)
	}
	fn := Wrap("Test", "test.action", func(x int) int { return x })
	RunWithCorrelationID("corr-in", func() { fn(3) })
	ob, _ := GetOutbox()
	events, _ := ob.Events()
	if events[0]["untrusted_payload"] != true {
		t.Fatalf("untrusted_payload: %v", events[0]["untrusted_payload"])
	}
}

func TestWrapPanicWithRecordFailureChainsError(t *testing.T) {
	dir := t.TempDir()
	if err := Configure(ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_panic_chain",
	}); err != nil {
		t.Fatal(err)
	}
	clientMu.Lock()
	instancePrivate = ed25519.PrivateKey{0}
	clientMu.Unlock()
	fn := Wrap("Test", "test.action", func(_ struct{}) int {
		panic("boom")
	})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
	}()
	RunWithCorrelationID("corr-pc", func() { fn(struct{}{}) })
}

func TestWrapFuncPanicWithRecordFailure(t *testing.T) {
	dir := t.TempDir()
	if err := Configure(ConfigureOptions{
		DBPath:   filepath.Join(dir, "outbox.db"),
		DataDir:  dir,
		TenantID: "tnt_wf_panic",
	}); err != nil {
		t.Fatal(err)
	}
	clientMu.Lock()
	instancePrivate = ed25519.PrivateKey{0}
	clientMu.Unlock()
	fn := WrapFunc("Test", "test.action", func(_ struct{}) (int, error) {
		panic("boom")
	})
	defer func() { recover() }()
	_, _ = fn(struct{}{})
}

func TestWrapFuncReturnsFnErrWhenRecordFails(t *testing.T) {
	resetRuntimeForTest()
	fn := WrapFunc("Test", "test.action", func(_ struct{}) (int, error) {
		return 1, os.ErrInvalid
	})
	_, err := fn(struct{}{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRecordExecutionGetInstanceIDFails(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	resetRuntimeForTest()
	clientMu.Lock()
	outbox = ob
	instanceID = ""
	instancePrivate = ed25519.PrivateKey{}
	tenantID = "tnt"
	clientMu.Unlock()
	if err := recordExecution("i", "a", "c", "e", 0, 1, nil, nil, "ok", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRecordExecutionGetPrivateKeyFails(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	resetRuntimeForTest()
	clientMu.Lock()
	outbox = ob
	instanceID = "inst_partial"
	instancePrivate = nil
	tenantID = "tnt"
	clientMu.Unlock()
	err = recordExecution("i", "a", "c", "e", 0, 1, nil, nil, "ok", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRecordExecutionRequiresConfigure(t *testing.T) {
	resetRuntimeForTest()
	err := recordExecution(
		"i", "a", "c", "e",
		0, 1, nil, nil, "ok", nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRecordExecutionWithExporter(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	if err := Configure(ConfigureOptions{
		DBPath:    filepath.Join(dir, "outbox.db"),
		DataDir:   dir,
		TenantID:  "tnt_exp",
		IngestURL: srv.URL,
	}); err != nil {
		t.Fatal(err)
	}
	fn := Wrap("Export", "export.test", func(x int) int { return x + 1 })
	RunWithCorrelationID("corr-exp", func() { fn(1) })
	Flush()
}

func TestWrapLogsRecordFailureOnSuccessPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "outbox.db")
	dataDir := filepath.Join(dir, "data")
	if err := Configure(ConfigureOptions{
		DBPath:   dbPath,
		DataDir:  dataDir,
		TenantID: "tnt_log_rec",
	}); err != nil {
		t.Fatal(err)
	}
	ob, err := GetOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if err := ob.Close(); err != nil {
		t.Fatal(err)
	}

	var logged atomic.Bool
	prev := logExecutionRecordFailure
	logExecutionRecordFailure = func(err error) {
		logged.Store(true)
		if err == nil {
			t.Fatal("expected record error")
		}
	}
	defer func() { logExecutionRecordFailure = prev }()

	fn := Wrap("Test", "test.action", func(x int) int { return x + 1 })
	if got := fn(2); got != 3 {
		t.Fatalf("result: got %d want 3", got)
	}
	if !logged.Load() {
		t.Fatal("expected record failure to be logged")
	}
}

func TestWrapFuncReturnsRecordError(t *testing.T) {
	resetRuntimeForTest()
	fn := WrapFunc("Test", "test.action", func(x int) (int, error) { return x, nil })
	_, err := fn(1)
	if err == nil {
		t.Fatal("expected record error")
	}
}

func TestRecordExecutionSignEventFailure(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	resetRuntimeForTest()
	seed := make([]byte, ed25519.SeedSize)
	priv := ed25519.NewKeyFromSeed(seed)
	clientMu.Lock()
	outbox = ob
	instanceID = "inst_sign"
	instancePrivate = priv
	tenantID = "tnt"
	clientMu.Unlock()

	// Force canonicalization failure inside buildSigned via invalid nested value.
	clientMu.Lock()
	instancePrivate = priv
	clientMu.Unlock()
	if err := recordExecution("i", "a", "c", "e", 0, 1, []any{make(chan int)}, nil, "ok", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestWrapFuncRecordFailureChained(t *testing.T) {
	resetRuntimeForTest()
	fn := WrapFunc("Test", "test.action", func(_ struct{}) (int, error) { return 1, nil })
	_, err := fn(struct{}{})
	if err == nil {
		t.Fatal("expected configure error")
	}
}
