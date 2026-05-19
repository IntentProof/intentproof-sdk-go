package intentproof

import (
	"path/filepath"
	"sync/atomic"
	"testing"
)

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
