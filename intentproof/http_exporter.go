package intentproof

import (
	"log"
	"os"
	"strings"
	"sync"
)

const defaultLocalIngestURL = "http://127.0.0.1:9787/v1/events"

// ResolveIngestURL picks the ingest endpoint from explicit config or env.
func ResolveIngestURL(explicit string) string {
	raw := strings.TrimSpace(explicit)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("INTENTPROOF_INGEST_URL"))
	}
	if raw != "" {
		return normalizeIngestURL(raw)
	}
	if strings.TrimSpace(os.Getenv("INTENTPROOF_USE_LOCAL_INGEST")) == "1" {
		return defaultLocalIngestURL
	}
	return ""
}

func normalizeIngestURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(trimmed, "/v1/events") {
		return trimmed
	}
	return trimmed + "/v1/events"
}

// HTTPExporter posts signed events to ingest in the background.
type HTTPExporter struct {
	ingestURL string
	lock      sync.Mutex
	pending   []*sync.WaitGroup
}

// NewHTTPExporter creates an exporter for ingestURL.
func NewHTTPExporter(ingestURL string) *HTTPExporter {
	return &HTTPExporter{ingestURL: ingestURL}
}

// Enqueue starts a background POST for event.
func (e *HTTPExporter) Enqueue(event map[string]any) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	e.lock.Lock()
	e.pending = append(e.pending, wg)
	e.lock.Unlock()
	go func() {
		defer wg.Done()
		if err := PostExecutionEvent(e.ingestURL, event); err != nil {
			log.Printf("[intentproof] ingest export failed: %v", err)
		}
	}()
}

// Flush waits for in-flight exports.
func (e *HTTPExporter) Flush() {
	e.lock.Lock()
	pending := append([]*sync.WaitGroup(nil), e.pending...)
	e.pending = nil
	e.lock.Unlock()
	for _, wg := range pending {
		wg.Wait()
	}
}
