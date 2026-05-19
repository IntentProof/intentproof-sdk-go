package intentproof

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveIngestURLNormalizesBaseURL(t *testing.T) {
	got := ResolveIngestURL("http://127.0.0.1:9787")
	want := "http://127.0.0.1:9787/v1/events"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveIngestURLPreservesEventsPath(t *testing.T) {
	raw := "http://127.0.0.1:9787/v1/events"
	if got := ResolveIngestURL(raw); got != raw {
		t.Fatalf("got %q", got)
	}
	if got := ResolveIngestURL(raw + "/"); got != raw {
		t.Fatalf("trailing slash: got %q", got)
	}
}

func TestResolveIngestURLEmptyWhenUnset(t *testing.T) {
	os.Unsetenv("INTENTPROOF_INGEST_URL")
	os.Unsetenv("INTENTPROOF_USE_LOCAL_INGEST")
	if got := ResolveIngestURL(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveIngestURLFromEnv(t *testing.T) {
	t.Setenv("INTENTPROOF_INGEST_URL", "http://127.0.0.1:9787")
	if got := ResolveIngestURL(""); got != "http://127.0.0.1:9787/v1/events" {
		t.Fatalf("got %q", got)
	}
}

func TestHTTPExporterFlushWaitsForPOST(t *testing.T) {
	var posted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exp := NewHTTPExporter(srv.URL)
	exp.Enqueue(map[string]any{"schema": "intentproof.event.v1", "event_id": "evt_1"})
	exp.Flush()
	if posted.Load() != 1 {
		t.Fatalf("posted: %d", posted.Load())
	}
}

func TestHTTPExporterPrunesCompletedPending(t *testing.T) {
	var done atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Millisecond)
		done.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exp := NewHTTPExporter(srv.URL)
	const n = 20
	for range n {
		exp.Enqueue(map[string]any{"schema": "intentproof.event.v1"})
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if int(done.Load()) == n {
			exp.lock.Lock()
			remaining := len(exp.pending)
			exp.lock.Unlock()
			if remaining == 0 {
				return
			}
			t.Fatalf("pending not pruned after completion: %d", remaining)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out: done=%d want %d", done.Load(), n)
}

func TestHTTPExporterSustainedEnqueueWithoutFlush(t *testing.T) {
	var posted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posted.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exp := NewHTTPExporter(srv.URL)
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			exp.Enqueue(map[string]any{"schema": "intentproof.event.v1"})
		}()
	}
	wg.Wait()
	exp.Flush()
	if posted.Load() != n {
		t.Fatalf("posted: %d want %d", posted.Load(), n)
	}
	exp.lock.Lock()
	remaining := len(exp.pending)
	exp.lock.Unlock()
	if remaining != 0 {
		t.Fatalf("pending after flush: %d", remaining)
	}
}

func TestHTTPExporterPostsJSONBody(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exp := NewHTTPExporter(srv.URL)
	exp.Enqueue(map[string]any{"schema": "intentproof.event.v1", "event_id": "evt_body"})
	exp.Flush()
	if got["event_id"] != "evt_body" {
		t.Fatalf("body: %+v", got)
	}
}
