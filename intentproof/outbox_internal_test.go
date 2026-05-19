package intentproof

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenOutboxCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "outbox.db")
	ob, err := OpenOutbox(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()
	if _, err := ob.RecordChainedEvent("c", "e1", func(pos int, prev string) (map[string]any, string, error) {
		return map[string]any{"chain_position": pos}, "sha256:x", nil
	}); err != nil {
		t.Fatal(err)
	}
	events, err := ob.Events()
	if err != nil || len(events) != 1 {
		t.Fatalf("events: %v err=%v", events, err)
	}
}

func TestEventsRejectsInvalidJSONBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outbox.db")
	ob, err := OpenOutbox(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()
	if _, err := ob.db.Exec(`INSERT INTO events (event_id, body) VALUES (?, ?)`, "bad", "not-json"); err != nil {
		t.Fatal(err)
	}
	if _, err := ob.Events(); err == nil {
		t.Fatal("expected events error")
	}
}

func TestRecordChainedEventChainInsertFailure(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
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
	// Force chains row to invalid state to provoke insert failure on second event.
	if _, err := ob.db.Exec(`UPDATE chains SET last_position = 'bad' WHERE correlation_id = 'c'`); err != nil {
		t.Fatal(err)
	}
	if _, err := ob.RecordChainedEvent("c", "e2", build); err == nil {
		t.Fatal("expected chain update error")
	}
}

func TestRecordChainedEventBeginTxFailure(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	_ = ob.Close()
	_, err = ob.RecordChainedEvent("c", "e", func(int, string) (map[string]any, string, error) {
		return map[string]any{}, "h", nil
	})
	if err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestRecordChainedEventDuplicateEventID(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()

	build := func(pos int, prev string) (map[string]any, string, error) {
		return map[string]any{"chain_position": pos}, "sha256:x", nil
	}
	if _, err := ob.RecordChainedEvent("c", "dup", build); err != nil {
		t.Fatal(err)
	}
	if _, err := ob.RecordChainedEvent("c", "dup", build); err == nil {
		t.Fatal("expected duplicate event_id error")
	}
}

func TestRecordChainedEventMarshalFailure(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()

	_, err = ob.RecordChainedEvent("c", "evt-bad", func(int, string) (map[string]any, string, error) {
		return map[string]any{"bad": make(chan int)}, "sha256:x", nil
	})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestOpenOutboxRejectsDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenOutbox(dir); err == nil {
		t.Fatal("expected open error")
	}
}

func TestNextChainLinkQueryFailure(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()
	if _, err := ob.db.Exec("DROP TABLE chains"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ob.nextChainLink("c"); err == nil {
		t.Fatal("expected query error")
	}
}

func TestRecordChainedEventBuildSignedFailure(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()
	_, err = ob.RecordChainedEvent("c", "e1", func(int, string) (map[string]any, string, error) {
		return nil, "", os.ErrInvalid
	})
	if err == nil {
		t.Fatal("expected buildSigned error")
	}
}

func TestEventsScanFailure(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()
	if _, err := ob.db.Exec(`INSERT INTO events (event_id, body) VALUES (?, ?)`, "x", 123); err != nil {
		t.Fatal(err)
	}
	if _, err := ob.Events(); err == nil {
		t.Fatal("expected scan error")
	}
}

func TestNextChainLinkSentinel(t *testing.T) {
	dir := t.TempDir()
	ob, err := OpenOutbox(filepath.Join(dir, "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()
	pos, prev, err := ob.nextChainLink("new-corr")
	if err != nil || pos != 1 || prev != SentinelPrevHash {
		t.Fatalf("pos=%d prev=%s err=%v", pos, prev, err)
	}
}
