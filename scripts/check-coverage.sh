#!/usr/bin/env bash
# Tiered statement coverage from a Go cover profile.
#
# Usage: check-coverage.sh [profile]
# Default profile: coverage.out
#
# Policy: scripts/coverage-tiers.conf

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONF="${ROOT}/scripts/coverage-tiers.conf"
PROFILE_PATH="${1:-coverage.out}"

if [[ ! -f "$CONF" ]]; then
  echo "coverage tiers config not found: $CONF" >&2
  exit 2
fi

# shellcheck disable=SC1090
source "$CONF"

if [[ -z "${TOTAL_MIN:-}" ]]; then
  echo "coverage-tiers.conf must set TOTAL_MIN" >&2
  exit 2
fi

if [[ ! -f "$PROFILE_PATH" ]]; then
  echo "coverage profile not found: $PROFILE_PATH" >&2
  exit 2
fi

rules_file="$(mktemp)"
exclude_file="$(mktemp)"
trap 'rm -f "$rules_file" "$exclude_file"' EXIT

if [[ -n "${EXCLUDE_PATH_FRAGMENTS:-}" ]]; then
  printf '%s\n' "${EXCLUDE_PATH_FRAGMENTS[@]}" >"$exclude_file"
fi

if [[ -n "${CRITICAL_RULES:-}" ]]; then
  printf '%s\n' "${CRITICAL_RULES[@]}" >"$rules_file"
elif [[ -n "${CRITICAL_MIN:-}" && ${#CRITICAL_PREFIXES[@]} -gt 0 ]]; then
  while IFS= read -r prefix; do
    [[ -n "$prefix" ]] || continue
    printf '%s:%s\n' "$CRITICAL_MIN" "$prefix"
  done < <(printf '%s\n' "${CRITICAL_PREFIXES[@]}") >"$rules_file"
else
  echo "coverage-tiers.conf must set CRITICAL_RULES or CRITICAL_MIN + CRITICAL_PREFIXES" >&2
  exit 2
fi

read -r TOTAL_COVERED TOTAL_STMTS <<EOF
$(awk -v exclude_file="$exclude_file" '
  function excluded(path,   i, line) {
    while ((getline line < exclude_file) > 0) {
      if (line != "" && index(path, line) > 0) return 1
    }
    close(exclude_file)
    return 0
  }
  NR > 1 {
    path = $1
    sub(/:.*$/, "", path)
    if (exclude_file != "" && excluded(path)) next
    stmts = $(NF - 1) + 0
    cnt = $NF + 0
    total += stmts
    if (cnt > 0) covered += stmts
  }
  END { print covered + 0, total + 0 }
' "$PROFILE_PATH")
EOF

if [[ -z "$TOTAL_STMTS" || "$TOTAL_STMTS" -eq 0 ]]; then
  echo "unable to read total coverage from $PROFILE_PATH" >&2
  exit 2
fi

report_threshold() {
  local label="$1" covered="$2" total="$3" min="$4"
  local pct
  pct="$(awk -v c="$covered" -v t="$total" 'BEGIN { printf "%.1f", 100 * c / t }')"
  echo "${label}: ${pct}% (${covered}/${total} statements), minimum ${min}%"
  if awk -v c="$covered" -v t="$total" -v min="$min" \
    'BEGIN { exit !(t > 0 && c * 100 >= t * min) }'; then
    echo "  PASS"
    return 0
  fi
  echo "  FAIL" >&2
  return 1
}

prefix_coverage() {
  local prefix="$1"
  awk -v prefix="$prefix" -v exclude_file="$exclude_file" '
    function excluded(path,   line) {
      while ((getline line < exclude_file) > 0) {
        if (line != "" && index(path, line) > 0) return 1
      }
      close(exclude_file)
      return 0
    }
    NR > 1 {
      path = $1
      sub(/:.*$/, "", path)
      if (exclude_file != "" && excluded(path)) next
      if (index(path, prefix) == 0) next
      stmts = $(NF - 1) + 0
      cnt = $NF + 0
      total += stmts
      if (cnt > 0) covered += stmts
    }
    END { print covered + 0, total + 0 }
  ' "$PROFILE_PATH"
}

fail=0
report_threshold "Total coverage" "$TOTAL_COVERED" "$TOTAL_STMTS" "$TOTAL_MIN" || fail=1

echo "Critical tiers:"
while IFS= read -r rule; do
  [[ -n "$rule" ]] || continue
  min="${rule%%:*}"
  prefix="${rule#*:}"
  read -r c t <<EOF
$(prefix_coverage "$prefix")
EOF
  if [[ "$t" -eq 0 ]]; then
    echo "  ${prefix} (min ${min}%): no statements in profile, skipped"
    continue
  fi
  report_threshold "  ${prefix}" "$c" "$t" "$min" || fail=1
done <"$rules_file"

if [[ "$fail" -ne 0 ]]; then
  echo "FAIL: coverage threshold not met" >&2
  exit 1
fi

echo "PASS: coverage thresholds met"
exit 0
