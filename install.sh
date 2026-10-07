#!/usr/bin/env bash
# Install Mockly from GitHub Releases.
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/dever-labs/mockly/main/install.sh | bash
#
# Environment variables:
#   MOCKLY_VERSION  - release tag to install (default: latest)
#   INSTALL_DIR     - directory to place the binary (default: /usr/local/bin)
#
set -euo pipefail

REPO="dever-labs/mockly"
VERSION="${MOCKLY_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── Detect OS ────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux"  ;;
  darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $OS" >&2
    echo "For Windows, use install.ps1 instead:" >&2
    echo "  irm https://raw.githubusercontent.com/${REPO}/main/install.ps1 | iex" >&2
    exit 1
    ;;
esac

# ── Detect arch ───────────────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# ── Resolve version ───────────────────────────────────────────────────────────
if [ "$VERSION" = "latest" ]; then
  echo "Resolving latest Mockly version..."
  VERSION=$(curl -sSfL \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": "(.*)".*/\1/')
fi

if [ -z "$VERSION" ]; then
  echo "Failed to resolve version. Set MOCKLY_VERSION explicitly." >&2
  exit 1
fi

# ── Download ──────────────────────────────────────────────────────────────────
BINARY_NAME="mockly-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
TMP_FILE=$(mktemp)
TMP_CHECKSUMS=$(mktemp)
trap 'rm -f "$TMP_FILE" "$TMP_CHECKSUMS"' EXIT

echo "Installing mockly ${VERSION} (${OS}/${ARCH})..."
curl -sSfL "$DOWNLOAD_URL" -o "$TMP_FILE"

# ── Verify checksum ───────────────────────────────────────────────────────────
curl -sSfL "$CHECKSUMS_URL" -o "$TMP_CHECKSUMS"

EXPECTED_SHA256=$(grep " ${BINARY_NAME}\$" "$TMP_CHECKSUMS" | awk '{print $1}' || true)
if [ -z "$EXPECTED_SHA256" ]; then
  echo "Failed to find a checksum for ${BINARY_NAME} in ${CHECKSUMS_URL}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SHA256=$(sha256sum "$TMP_FILE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL_SHA256=$(shasum -a 256 "$TMP_FILE" | awk '{print $1}')
else
  echo "Neither sha256sum nor shasum is available; cannot verify the download." >&2
  exit 1
fi

if [ "$ACTUAL_SHA256" != "$EXPECTED_SHA256" ]; then
  echo "Checksum mismatch for ${BINARY_NAME}!" >&2
  echo "  expected: ${EXPECTED_SHA256}" >&2
  echo "  actual:   ${ACTUAL_SHA256}" >&2
  exit 1
fi
echo "Checksum verified."

# ── Install ───────────────────────────────────────────────────────────────────
chmod +x "$TMP_FILE"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_FILE" "${INSTALL_DIR}/mockly"
else
  sudo mv "$TMP_FILE" "${INSTALL_DIR}/mockly"
fi

echo "mockly ${VERSION} installed to ${INSTALL_DIR}/mockly"
