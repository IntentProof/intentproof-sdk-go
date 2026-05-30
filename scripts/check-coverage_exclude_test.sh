#!/usr/bin/env bash
# Regression test: EXCLUDE_PATH_FRAGMENTS must apply to every profile line.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
awk_agg="${script_dir}/check-coverage-aggregate.awk"

profile="$(mktemp)"
exclude="$(mktemp)"
trap 'rm -f "$profile" "$exclude"' EXIT

cat >"$profile" <<'EOF'
mode: set
example.com/pkg/jcs.go:1.1,2.1 1 1
example.com/pkg/jcs.go:3.1,4.1 1 0
example.com/pkg/main.go:1.1,2.1 2 2
example.com/pkg/main.go:3.1,4.1 2 0
EOF

printf '%s\n' "/jcs.go" >"$exclude"

read -r covered total <<EOF
$(awk -v exclude_file="$exclude" -v path_prefix="" -f "$awk_agg" "$profile")
EOF

if [[ "$covered" != "2" || "$total" != "4" ]]; then
  echo "FAIL: expected 2/4 covered after exclude, got ${covered}/${total}" >&2
  exit 1
fi

echo "PASS: exclude patterns apply on every profile line"
