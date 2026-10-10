#!/usr/bin/env bash
# Fails if total statement coverage across the internal/... packages (as
# reported in coverage.txt, produced by `go test -coverprofile`) drops below
# a floor. This guards against silent coverage regressions; it intentionally
# sits a few points below the coverage level at the time this script was
# added (~79%) to leave headroom for legitimate, lightly-tested additions
# (e.g. new protocol scaffolding) without needing to bump the threshold on
# every PR.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

THRESHOLD="${COVERAGE_THRESHOLD:-70}"
COVERAGE_FILE="${1:-coverage.txt}"

if [[ ! -f "$COVERAGE_FILE" ]]; then
  echo "error: coverage file '$COVERAGE_FILE' not found (run 'go test -coverprofile=$COVERAGE_FILE ...' first)" >&2
  exit 1
fi

total_line="$(go tool cover -func="$COVERAGE_FILE" | tail -1)"
# Last field looks like "79.1%"
pct="$(echo "$total_line" | awk '{print $NF}' | tr -d '%')"

if [[ -z "$pct" ]]; then
  echo "error: could not parse coverage percentage from: $total_line" >&2
  exit 1
fi

echo "Total coverage: ${pct}% (threshold: ${THRESHOLD}%)"

# Compare as integers (bash has no float arithmetic); truncate decimals.
pct_int="${pct%%.*}"
if (( pct_int < THRESHOLD )); then
  echo "error: coverage ${pct}% is below the ${THRESHOLD}% threshold" >&2
  exit 1
fi

echo "OK: coverage meets the ${THRESHOLD}% threshold."
