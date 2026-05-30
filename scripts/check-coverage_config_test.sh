#!/usr/bin/env bash
# Regression test: missing critical tier config yields a clear error.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

run_config_case() {
  local conf_body="$1"
  local label="$2"

  local root
  root="$(mktemp -d)"
  # shellcheck disable=SC2064
  trap "rm -rf '$root'" RETURN

  mkdir -p "$root/scripts"
  cp "$script_dir/check-coverage.sh" "$script_dir/check-coverage-aggregate.awk" \
    "$root/scripts/"

  cat >"$root/scripts/coverage-tiers.conf" <<EOF
${conf_body}
EOF

  cat >"$root/coverage.out" <<'EOF'
mode: atomic
example.com/pkg/main.go:1.1,2.1 2 2
EOF

  local output=""
  local code=0
  output="$(bash "$root/scripts/check-coverage.sh" "$root/coverage.out" 2>&1)" || code=$?

  if [[ "$code" -ne 2 ]]; then
    echo "FAIL (${label}): expected exit 2, got ${code}" >&2
    echo "$output" >&2
    return 1
  fi

  if ! grep -q "must set CRITICAL_RULES or CRITICAL_MIN + CRITICAL_PREFIXES" <<<"$output"; then
    echo "FAIL (${label}): expected descriptive config error, got:" >&2
    echo "$output" >&2
    return 1
  fi

  return 0
}

run_config_case "TOTAL_MIN=90" "no critical tier keys" || exit 1
run_config_case $'TOTAL_MIN=90\nCRITICAL_MIN=95' "CRITICAL_MIN without CRITICAL_PREFIXES" || exit 1

echo "PASS: incomplete critical tier config reports clear error"
