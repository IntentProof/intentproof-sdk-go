#!/usr/bin/env bash
# CI-parity coverage gate for local checkpoints and manual runs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

GOWORK=off go test -covermode=set -coverprofile=coverage.out ./intentproof/...
exec bash ./scripts/check-coverage.sh coverage.out
