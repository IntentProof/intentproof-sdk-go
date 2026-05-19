package intentproof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var ingestHTTPClient = &http.Client{Timeout: 5 * time.Second}

// IngestRequestHeaders returns HTTP headers for ingest POSTs.
func IngestRequestHeaders() map[string]string {
	headers := map[string]string{"Content-Type": "application/json"}
	if token := os.Getenv("INTENTPROOF_INGEST_TOKEN"); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

func postExecutionEventBody(ingestURL string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, ingestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range IngestRequestHeaders() {
		req.Header.Set(k, v)
	}
	resp, err := ingestHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("ingest POST %d: %s", resp.StatusCode, string(detail))
	}
	return nil
}

// PostExecutionEvent POSTs a signed event to ingest.
func PostExecutionEvent(ingestURL string, event map[string]any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return postExecutionEventBody(ingestURL, body)
}
