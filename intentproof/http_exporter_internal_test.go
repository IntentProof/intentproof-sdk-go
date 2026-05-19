package intentproof

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPExporterEnqueueSnapshotBeforeAsync(t *testing.T) {
	var posted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["event_id"] != "evt_snap" {
			t.Fatalf("event_id: %v", body["event_id"])
		}
		posted.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exp := NewHTTPExporter(srv.URL)
	event := map[string]any{"schema": "intentproof.event.v1", "event_id": "evt_snap"}
	exp.Enqueue(event)
	event["event_id"] = "mutated"
	exp.Flush()
	if posted.Load() != 1 {
		t.Fatalf("posted: %d", posted.Load())
	}
}

func TestHTTPExporterFlushWaitsAfterDone(t *testing.T) {
	block := make(chan struct{})
	var posted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-block
		posted.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exp := NewHTTPExporter(srv.URL)
	exp.Enqueue(map[string]any{"schema": "intentproof.event.v1"})
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		exp.Flush()
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	close(block)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("flush returned before export completed")
	}
	if posted.Load() != 1 {
		t.Fatalf("posted: %d", posted.Load())
	}
}
