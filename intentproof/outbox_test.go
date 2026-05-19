package intentproof_test

import (
	"errors"
	"testing"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func TestOutboxChainContinuity(t *testing.T) {
	dbPath, _ := testDirs(t)
	ob, err := intentproof.OpenOutbox(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()

	build := func(pos int, prev string) (map[string]any, string, error) {
		ev := map[string]any{
			"correlation_id":  "corr-outbox",
			"chain_position":  pos,
			"prev_event_hash": prev,
		}
		return ev, "sha256:abc", nil
	}
	if _, err := ob.RecordChainedEvent("corr-outbox", "evt-1", build); err != nil {
		t.Fatal(err)
	}
	ev2, err := ob.RecordChainedEvent("corr-outbox", "evt-2", build)
	if err != nil {
		t.Fatal(err)
	}
	if ev2["chain_position"] != 2 {
		t.Fatalf("position: %v", ev2["chain_position"])
	}
}

func TestOutboxRecordChainedEventPropagatesBuildError(t *testing.T) {
	dbPath, _ := testDirs(t)
	ob, err := intentproof.OpenOutbox(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()

	buildErr := errors.New("sign failed")
	_, err = ob.RecordChainedEvent("corr", "evt", func(int, string) (map[string]any, string, error) {
		return nil, "", buildErr
	})
	if !errors.Is(err, buildErr) {
		t.Fatalf("err: %v", err)
	}
}
