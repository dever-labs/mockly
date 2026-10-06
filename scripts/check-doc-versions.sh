#!/usr/bin/env bash
# Verifies that hardcoded third-party dependency versions quoted in
# documentation match the authoritative version pinned in the relevant
# client package manifest.
#
# Mockly's own release version is tracked via the `x-release-please-version`
# marker and kept in sync automatically by release-please. Versions of
# dependencies *of* the client libraries (e.g. the Rust `testcontainers`
# crate pinned in clients/rust-testcontainers/Cargo.toml) have no such
# mechanism, so they silently drift whenever the manifest is bumped
# (including by Dependabot) without a matching doc update. This script
# catches that drift in CI.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

fail=0

check() {
  local label="$1" manifest="$2" manifest_pattern="$3" doc="$4" doc_pattern="$5"

  local manifest_version doc_version
  manifest_version=$(grep -oP "$manifest_pattern" "$manifest" | head -1)
  doc_version=$(grep -oP "$doc_pattern" "$doc" | head -1)

  if [[ -z "$manifest_version" || -z "$doc_version" ]]; then
    echo "::error::$label: could not extract a version from $manifest or $doc (pattern may need updating)"
    fail=1
    return
  fi

  if [[ "$manifest_version" != "$doc_version" ]]; then
    echo "::error::$label: version drift — $manifest has '$manifest_version' but $doc has '$doc_version'"
    fail=1
  fi
}

check "docs/clients/rust.md: testcontainers crate" \
  clients/rust-testcontainers/Cargo.toml 'testcontainers = \{ version = "\K[^"]+' \
  docs/clients/rust.md 'testcontainers = \{ version = "\K[^"]+'

check "clients/rust-testcontainers/README.md: testcontainers crate" \
  clients/rust-testcontainers/Cargo.toml 'testcontainers = \{ version = "\K[^"]+' \
  clients/rust-testcontainers/README.md 'testcontainers = \{ version = "\K[^"]+'

if [[ "$fail" -ne 0 ]]; then
  echo
  echo "One or more documented dependency versions are out of sync with their source manifest."
  echo "Update the doc(s) above to match, then re-run this check."
  exit 1
fi

echo "All documented dependency versions match their source manifests."
