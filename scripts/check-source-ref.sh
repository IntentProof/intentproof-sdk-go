#!/usr/bin/env bash
# Fail when SOURCE_REF does not match the current commit (tuple bookkeeping).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f SOURCE_REF ]]; then
  echo "SKIP: SOURCE_REF not present yet (add when bumping the ecosystem tuple)."
  exit 0
fi

HEAD="$(git rev-parse HEAD)"
REF="$(tr -d '[:space:]' < SOURCE_REF)"
if ! echo "$REF" | grep -qE '^[0-9a-f]{40}$'; then
  echo "Invalid SOURCE_REF: must be a 40-character lowercase git SHA" >&2
  exit 1
fi

if [[ "$HEAD" == "$REF" ]]; then
  echo "PASS: SOURCE_REF matches HEAD ($HEAD)."
  exit 0
fi

if git merge-base --is-ancestor "$REF" "$HEAD" 2>/dev/null; then
  echo "FAIL: HEAD ($HEAD) is ahead of SOURCE_REF ($REF)" >&2
  echo "Bump SOURCE_REF to HEAD and update spec pins/matrix for the tuple." >&2
  exit 1
fi

echo "FAIL: SOURCE_REF ($REF) is not an ancestor of HEAD ($HEAD)" >&2
exit 1
