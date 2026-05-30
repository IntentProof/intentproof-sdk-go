# Tiered coverage policy (intentproof-sdk-go)

The SDK is profiled once (`./intentproof/...` with `jcs.go` excluded). The
two tiers apply **different minimums to overlapping scope**, not separate
measurement passes:

| Tier | Minimum | Role |
|------|---------|------|
| **Total** | 90% | Repo-wide floor for all profiled SDK code |
| **Critical** | 95% | Stricter floor on the public `intentproof/` package |

Critical is intentionally the same tree as total here: the public SDK surface
must stay at **95%** even when the overall repo total only requires **90%**.
If total coverage is healthy but the SDK package slips toward 90%, the critical
tier still fails.

Configuration: `scripts/coverage-tiers.conf`. Enforcement:
`scripts/check-coverage.sh`.
