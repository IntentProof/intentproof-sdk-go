# intentproof-sdk-go

Go SDK for emitting signed IntentProof execution events.

## Status

Early scaffolding repo for IntentProof's Go SDK. Tracks the Node and
Python SDK `wrap()` / exporter / outbox contract so a Go application
can emit and verify the same signed execution events.

## Planned scope

- `wrap()` instrumentation helper
- Correlation-id helpers
- Event signing and canonical serialization (JCS)
- Durable outbox and hosted ingest transport

## Module path

```text
github.com/intentproof/intentproof-sdk-go
```

SDK implementation and local development steps land in a follow-on
change.

## License

Apache License 2.0 (`LICENSE`).
