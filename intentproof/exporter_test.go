package intentproof_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestConfigureWrapFlushPostsToIngest(t *testing.T) {
	var posted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posted.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:    filepath.Join(dir, "outbox.db"),
		DataDir:   filepath.Join(dir, "data"),
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
