# Shared go cover profile aggregation for check-coverage.sh.
# Variables: exclude_file (optional path), path_prefix (optional filter).
BEGIN {
  if (exclude_file != "") {
    while ((getline line < exclude_file) > 0) {
      if (line != "") excl[++n] = line
    }
    close(exclude_file)
  }
}
function excluded(path,   i) {
  for (i = 1; i <= n; i++) {
    if (index(path, excl[i]) > 0) return 1
  }
  return 0
}
NR > 1 {
  path = $1
  sub(/:.*$/, "", path)
  if (n > 0 && excluded(path)) next
  if (path_prefix != "" && index(path, path_prefix) == 0) next
  stmts = $(NF - 1) + 0
  cnt = $NF + 0
  total += stmts
  if (cnt > 0) covered += stmts
}
END { print covered + 0, total + 0 }
