# intentproof-sdk-go

[![CI](https://github.com/IntentProof/intentproof-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/IntentProof/intentproof-sdk-go/actions/workflows/ci.yml)

Go SDK for emitting signed IntentProof execution events.

## Who uses this

Go application authors who instrument business logic with `Wrap` and export
signed execution events to local or hosted ingest.

## Status

Core SDK aligned with the Node and Python SDKs: `Configure`, `Wrap`,
`RunWithCorrelationID`, `Flush`, JCS canonicalization, Ed25519 signing,
SQLite WAL outbox, and optional HTTP export to ingest.

## Module path

```text
github.com/intentproof/intentproof-sdk-go
```

## Install

```bash
go get github.com/intentproof/intentproof-sdk-go/intentproof
```

## Verify

Cross-language signing fixtures under `testdata/fixtures/` match the Node and
Python SDK conformance set. Run `go test ./...` before tagging releases.

## Test

```bash
go test ./...
GOWORK=off go test -coverprofile=coverage.out ./intentproof/...
bash ./scripts/check-coverage.sh coverage.out
```

Tiered coverage: **90%** total and **95%** on `intentproof/` (see
`scripts/README-coverage-tiers.md`). The vendored RFC 8785 engine in `jcs.go` is
validated by conformance vectors in `jcs_test.go` and is excluded from that
line threshold.

## Release

Go module tags are published from this repository. Maintainer binary releases
(if any) use Sigstore signing via
[`intentproof-tools`](https://github.com/IntentProof/intentproof-tools).

## Documentation hub

Per-repo README files plus
[`intentproof-infra`](https://github.com/IntentProof/intentproof-infra) for
self-host install and image verification. Docs site deferred — see
[`docs-hub-decision.md`](https://github.com/IntentProof/intentproof-infra/blob/main/docs/docs-hub-decision.md).

## Support

Report bugs, API gaps, and conformance findings via
[GitHub Issues](https://github.com/IntentProof/intentproof-sdk-go/issues).
See [`CONTRIBUTING.md`](CONTRIBUTING.md). Security reports:
[`SECURITY.md`](SECURITY.md).

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

## License

Apache License 2.0 — see [`LICENSE`](LICENSE), [`NOTICE`](NOTICE), and
[`TRADEMARK.md`](TRADEMARK.md).
