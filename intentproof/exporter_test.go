package intentproof_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func TestIngestRequestHeadersBearer(t *testing.T) {
	t.Setenv("INTENTPROOF_INGEST_TOKEN", "ingest-secret")
	headers := intentproof.IngestRequestHeaders()
	if headers["Authorization"] != "Bearer ingest-secret" {
		t.Fatalf("Authorization: %q", headers["Authorization"])
	}
}

func TestIngestRequestHeadersOmitsBearerWithoutToken(t *testing.T) {
	os.Unsetenv("INTENTPROOF_INGEST_TOKEN")
	headers := intentproof.IngestRequestHeaders()
	if _, ok := headers["Authorization"]; ok {
		t.Fatal("expected no Authorization header")
	}
}

func TestPostExecutionEventAccepts202(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	event := map[string]any{"schema": "intentproof.event.v1"}
	if err := intentproof.PostExecutionEvent(srv.URL, event); err != nil {
		t.Fatal(err)
	}
}

func TestPostExecutionEventHTTPErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()

	err := intentproof.PostExecutionEvent(srv.URL, map[string]any{"schema": "intentproof.event.v1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPostExecutionEventRejects500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	err := intentproof.PostExecutionEvent(srv.URL, map[string]any{"schema": "intentproof.event.v1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveIngestURLFromEnv(t *testing.T) {
	t.Setenv("INTENTPROOF_INGEST_URL", "https://ingest.example.com")
	got := intentproof.ResolveIngestURL("")
	if got != "https://ingest.example.com/v1/events" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveIngestURLLocalFlag(t *testing.T) {
	os.Unsetenv("INTENTPROOF_INGEST_URL")
	t.Setenv("INTENTPROOF_USE_LOCAL_INGEST", "1")
	got := intentproof.ResolveIngestURL("")
	if got != "http://127.0.0.1:9787/v1/events" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveIngestURLExplicitAppendsPath(t *testing.T) {
	got := intentproof.ResolveIngestURL("https://host.example")
	if got != "https://host.example/v1/events" {
		t.Fatalf("got %q", got)
	}
}

func TestHTTPExporterEnqueueAndFlush(t *testing.T) {
	var posted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exp := intentproof.NewHTTPExporter(srv.URL)
	exp.Enqueue(map[string]any{"schema": "intentproof.event.v1"})
	exp.Flush()
	if posted.Load() != 1 {
		t.Fatalf("posted: %d", posted.Load())
	}
}

func TestPostExecutionEventMarshalFailure(t *testing.T) {
	err := intentproof.PostExecutionEvent("http://127.0.0.1:1/v1/events", map[string]any{
		"bad": make(chan int),
	})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestPostExecutionEventInvalidURL(t *testing.T) {
	err := intentproof.PostExecutionEvent("://not-a-valid-url", map[string]any{"schema": "intentproof.event.v1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPostExecutionEventConnectionFailure(t *testing.T) {
	err := intentproof.PostExecutionEvent(
		"http://127.0.0.1:1/v1/events",
		map[string]any{"schema": "intentproof.event.v1"},
	)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestDecodeJSONMapInvalidJSON(t *testing.T) {
	_, err := intentproof.DecodeJSONMap([]byte("{"))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestConfigureWithIngestURL(t *testing.T) {
	var posted atomic.Int32
	dbPath, dataDir := testDirs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posted.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:    dbPath,
		DataDir:   dataDir,
		TenantID:  "tnt_export",
		IngestURL: srv.URL,
	}); err != nil {
		t.Fatal(err)
	}
	fn := intentproof.Wrap("Export", "export.test", func(n int) int { return n + 1 })
	intentproof.RunWithCorrelationID("corr-export", func() { fn(1) })
	intentproof.Flush()
	if posted.Load() != 1 {
		t.Fatalf("posted: %d", posted.Load())
	}
}
