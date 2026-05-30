#!/usr/bin/env bash
# Fail when mirrored SDK signing fixtures drift from intentproof-spec.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
LOCAL="${ROOT}/testdata/fixtures"

spec_dir="${INTENTPROOF_SPEC_DIR:?INTENTPROOF_SPEC_DIR must point at intentproof-spec}"
if [[ "$spec_dir" != /* ]]; then
  spec_dir="${ROOT}/${spec_dir}"
fi
CANONICAL="${spec_dir}/golden/sdk-signing"

if [[ ! -d "$CANONICAL" ]]; then
  echo "canonical sdk-signing fixtures not found at ${CANONICAL}" >&2
  exit 1
fi

shopt -s nullglob
files=("${CANONICAL}"/signing_*)
if [[ ${#files[@]} -eq 0 ]]; then
  echo "no signing fixtures under ${CANONICAL}" >&2
  exit 1
fi

fail=0
for canonical in "${files[@]}"; do
  base="$(basename "$canonical")"
  local_path="${LOCAL}/${base}"
  if [[ ! -f "$local_path" ]]; then
    continue
  fi
  if ! cmp -s "$canonical" "$local_path"; then
    echo "sdk-signing fixture drift: ${base} differs from spec golden/sdk-signing" >&2
    fail=1
  fi
done

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi

echo "PASS: sdk-signing fixture mirrors match spec golden."
