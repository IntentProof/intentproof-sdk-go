package intentproof

import (
	"encoding/json"
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
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("[intentproof] ingest export failed: %v", err)
		return
	}
	url := e.ingestURL
	wg := &sync.WaitGroup{}
	wg.Add(1)
	e.lock.Lock()
	e.pending = append(e.pending, wg)
	e.lock.Unlock()
	go func() {
		defer e.dropPending(wg)
		defer wg.Done()
		if err := postExecutionEventBody(url, body); err != nil {
			log.Printf("[intentproof] ingest export failed: %v", err)
		}
	}()
}

func (e *HTTPExporter) dropPending(wg *sync.WaitGroup) {
	e.lock.Lock()
	defer e.lock.Unlock()
	for i, p := range e.pending {
		if p == wg {
			e.pending = append(e.pending[:i], e.pending[i+1:]...)
			return
		}
	}
}

// Flush waits for in-flight exports that were pending when Flush began.
func (e *HTTPExporter) Flush() {
	e.lock.Lock()
	snapshot := append([]*sync.WaitGroup(nil), e.pending...)
	e.lock.Unlock()
	for _, wg := range snapshot {
		wg.Wait()
	}
}
