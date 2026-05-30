#!/usr/bin/env bash
# Tiered statement coverage from a Go cover profile.
#
# Usage: check-coverage.sh [profile]
# Default profile: coverage.out
#
# Policy: scripts/coverage-tiers.conf

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPTS="${ROOT}/scripts"
CONF="${SCRIPTS}/coverage-tiers.conf"
AWK_AGG="${SCRIPTS}/check-coverage-aggregate.awk"
PROFILE_PATH="${1:-coverage.out}"

if [[ ! -f "$CONF" ]]; then
  echo "coverage tiers config not found: $CONF" >&2
  exit 2
fi

if [[ ! -f "$AWK_AGG" ]]; then
  echo "coverage awk helper not found: $AWK_AGG" >&2
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
elif [[ -n "${CRITICAL_MIN:-}" ]]; then
  if declare -p CRITICAL_PREFIXES &>/dev/null && [[ ${#CRITICAL_PREFIXES[@]} -gt 0 ]]; then
    while IFS= read -r prefix; do
      [[ -n "$prefix" ]] || continue
      printf '%s:%s\n' "$CRITICAL_MIN" "$prefix"
    done < <(printf '%s\n' "${CRITICAL_PREFIXES[@]}") >"$rules_file"
  else
    echo "coverage-tiers.conf must set CRITICAL_RULES or CRITICAL_MIN + CRITICAL_PREFIXES" >&2
    exit 2
  fi
else
  echo "coverage-tiers.conf must set CRITICAL_RULES or CRITICAL_MIN + CRITICAL_PREFIXES" >&2
  exit 2
fi

profile_coverage() {
  local prefix="${1:-}"
  awk -v exclude_file="$exclude_file" -v path_prefix="$prefix" \
    -f "$AWK_AGG" "$PROFILE_PATH"
}

coverage_percent_display() {
  awk -v c="$1" -v t="$2" 'BEGIN {
    if (t == 0) { print "0.0"; exit }
    printf "%.1f", int(1000 * c / t) / 10
  }'
}

threshold_met() {
  awk -v c="$1" -v t="$2" -v min="$3" \
    'BEGIN { exit !(t > 0 && c * 100 >= t * min) }'
}

read -r TOTAL_COVERED TOTAL_STMTS <<EOF
$(profile_coverage "")
EOF

if [[ -z "$TOTAL_STMTS" || "$TOTAL_STMTS" -eq 0 ]]; then
  echo "unable to read total coverage from $PROFILE_PATH" >&2
  exit 2
fi

report_threshold() {
  local label="$1" covered="$2" total="$3" min="$4"
  local pct
  pct="$(coverage_percent_display "$covered" "$total")"
  echo "${label}: ${pct}% (${covered}/${total} statements), minimum ${min}%"
  if threshold_met "$covered" "$total" "$min"; then
    echo "  PASS"
    return 0
  fi
  echo "  FAIL" >&2
  return 1
}

fail=0
report_threshold "Total coverage" "$TOTAL_COVERED" "$TOTAL_STMTS" "$TOTAL_MIN" || fail=1

echo "Critical tiers:"
while IFS= read -r rule; do
  [[ -n "$rule" ]] || continue
  min="${rule%%:*}"
  prefix="${rule#*:}"
  read -r c t <<EOF
$(profile_coverage "$prefix")
EOF
  if [[ "$t" -eq 0 ]]; then
    echo "  ${prefix} (min ${min}%): no statements in profile, FAIL" >&2
    fail=1
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
