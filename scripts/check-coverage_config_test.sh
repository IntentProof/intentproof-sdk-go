#!/usr/bin/env bash
# Regression test: missing critical tier config yields a clear error.
set -euo pipefail

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

mkdir -p "$root/scripts"
cp "$(dirname "$0")/check-coverage.sh" "$(dirname "$0")/check-coverage-aggregate.awk" \
  "$root/scripts/"

cat >"$root/scripts/coverage-tiers.conf" <<'EOF'
TOTAL_MIN=90
EOF

cat >"$root/coverage.out" <<'EOF'
mode: atomic
example.com/pkg/main.go:1.1,2.1 2 2
EOF

output=""
code=0
output="$(bash "$root/scripts/check-coverage.sh" "$root/coverage.out" 2>&1)" || code=$?

if [[ "$code" -ne 2 ]]; then
  echo "FAIL: expected exit 2 for incomplete config, got ${code}" >&2
  echo "$output" >&2
  exit 1
fi

if ! grep -q "must set CRITICAL_RULES or CRITICAL_MIN + CRITICAL_PREFIXES" <<<"$output"; then
  echo "FAIL: expected descriptive config error, got:" >&2
  echo "$output" >&2
  exit 1
fi

echo "PASS: incomplete critical tier config reports clear error"
