# intentproof-sdk-go

Go SDK for emitting signed IntentProof execution events.

## Status

Core SDK aligned with the Node and Python SDKs: `Configure`, `Wrap`,
`RunWithCorrelationID`, `Flush`, JCS canonicalization, Ed25519 signing,
SQLite WAL outbox, and optional HTTP export to ingest.

## Module path

```text
github.com/intentproof/intentproof-sdk-go
```

## Quick start

```go
package main

import (
	"log"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func main() {
	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:  "./intentproof-outbox.db",
		DataDir: "./.intentproof-sdk-go",
	}); err != nil {
		log.Fatal(err)
	}

	refund := intentproof.Wrap(
		"Return funds to the customer",
		"payments.refund.execute",
		func(input map[string]any) map[string]any {
			// call your payment provider here
			return map[string]any{"id": "re_123"}
		},
	)

	intentproof.RunWithCorrelationID("req_refund_ord_1042", func() {
		_ = refund(map[string]any{
			"amount_cents":   4999,
			"payment_intent": "pi_123",
		})
	})

	intentproof.Flush()
}
```

## Local ingest

- `INTENTPROOF_INGEST_URL` — hosted or local ingest base URL (normalized to
  `/v1/events`).
- `INTENTPROOF_USE_LOCAL_INGEST=1` — default local loop
  `http://127.0.0.1:9787/v1/events`.
- `INTENTPROOF_INGEST_TOKEN` — bearer token for hosted ingest.
- `INTENTPROOF_TENANT_ID` — default tenant when `Configure` omits `TenantID`.
- `INTENTPROOF_OUTBOX_PATH` — SQLite outbox path when `DBPath` is omitted.

Default signing keys live under `~/.intentproof/sdk-go/keypair.json`.

## Development

```bash
go test ./...
bash ./scripts/check-coverage.sh 95
```

CI enforces at least 95% line coverage on the `intentproof/` package (see
`scripts/check-coverage.sh`). The vendored RFC 8785 engine in `jcs.go` is
validated by conformance vectors in `jcs_test.go` and is excluded from that
line threshold.

Cross-language signing fixtures under `testdata/fixtures/` match the Node and
Python SDK conformance set.

## License

Apache License 2.0 (`LICENSE`).
