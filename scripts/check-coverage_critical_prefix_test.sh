#!/usr/bin/env bash
# Regression test: critical tiers must fail when a prefix matches nothing.
set -euo pipefail

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

mkdir -p "$root/scripts"
cp "$(dirname "$0")/check-coverage.sh" "$root/scripts/"

cat >"$root/scripts/coverage-tiers.conf" <<'EOF'
TOTAL_MIN=50
CRITICAL_RULES=(
  "95:/pkg/missing/"
)
EOF

cat >"$root/coverage.out" <<'EOF'
mode: atomic
example.com/pkg/verifier/v.go:1.1,2.1 2 2
EOF

if bash "$root/scripts/check-coverage.sh" "$root/coverage.out" >/dev/null 2>&1; then
  echo "FAIL: expected gate to fail for missing critical prefix" >&2
  exit 1
fi

echo "PASS: missing critical prefix fails the gate"
