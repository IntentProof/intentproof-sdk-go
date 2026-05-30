# intentproof-sdk-go

[![CI](https://github.com/IntentProof/intentproof-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/IntentProof/intentproof-sdk-go/actions/workflows/ci.yml)

Go SDK for signing IntentProof execution events locally.

## Use

- `Configure`, `Wrap`, `RunWithCorrelationID`, `Flush`
- JCS canonicalization and Ed25519 signing
- SQLite WAL outbox
- Optional HTTP export for local dev loops only

## Module

```text
github.com/intentproof/intentproof-sdk-go
```

## Install

```bash
go get github.com/intentproof/intentproof-sdk-go/intentproof
```

## Test

```bash
go test ./...
```

Fixtures align with
[`intentproof-spec`](https://github.com/IntentProof/intentproof-spec).

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
			return map[string]any{"id": "re_123"}
		},
	)

	intentproof.RunWithCorrelationID("req_refund_ord_1042", func() {
		_ = refund(map[string]any{"amount_cents": 4999})
	})

	intentproof.Flush()
}
```

Default keys: `~/.intentproof/sdk-go/keypair.json`.

## Support

[GitHub Issues](https://github.com/IntentProof/intentproof-sdk-go/issues) —
see [CONTRIBUTING.md](CONTRIBUTING.md). Security reports:
`security@intentproof.io` or a private GitHub Security Advisory.

## License

MIT — see [LICENSE](LICENSE).
