#!/usr/bin/env bash

set -euo pipefail

MIN_COVERAGE="${1:-95}"

go test -covermode=set -coverprofile=coverage.out ./intentproof/...

TOTAL_PERCENT="$(awk '
  !/^mode:/ {
    n = split($0, a, " ")
    stmt = a[n - 1] + 0
    cnt = a[n] + 0
    if ($0 ~ /intentproof-sdk-go\/intentproof\// && $0 !~ /jcs\.go:/) {
      total += stmt
      if (cnt > 0) {
        covered += stmt
      }
    }
  }
  END {
    if (total == 0) {
      exit 2
    }
    printf "%.1f", (100 * covered) / total
  }
' coverage.out)"

echo "SDK coverage (intentproof/, excludes vendored jcs.go): ${TOTAL_PERCENT}%"
echo "Minimum required: ${MIN_COVERAGE}%"
echo "(jcs.go is covered by conformance tests in jcs_test.go.)"

if awk -v got="$TOTAL_PERCENT" -v min="$MIN_COVERAGE" 'BEGIN { exit !(got + 0 >= min + 0) }'; then
  echo "PASS: coverage threshold met"
  exit 0
fi

echo "FAIL: coverage threshold not met" >&2
exit 1
