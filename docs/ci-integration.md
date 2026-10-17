# CI Integration

Mockly is a single static binary with no runtime dependencies — ideal for CI.

## GitHub Actions

```yaml
steps:
  - uses: actions/checkout@v5

  - name: Start Mockly
    id: mockly
    uses: dever-labs/mockly/.github/actions/setup-mockly@v0.15.0 # x-release-please-version
    with:
      version: v0.15.0         # x-release-please-version
      config: mockly.yaml      # path to your config
      api-port: 9091           # management API port (default)
      ready-timeout: 30        # seconds to wait for the API to come up (default)

  - name: Run tests
    run: npm test

  - name: Upload mockly logs on failure
    if: failure()
    uses: actions/upload-artifact@v4
    with:
      name: mockly-logs
      path: ${{ steps.mockly.outputs.log-file }}
```

The action verifies the downloaded binary's checksum against the release's
`checksums.txt`, and automatically:
- Downloads the right binary for the runner OS/arch
- Starts mockly in the background, capturing its stdout/stderr to the file
  exposed as the `log-file` output
- Waits up to `ready-timeout` seconds (default 30) for the server to be ready
- Stops mockly via a genuine `post` step, which GitHub Actions guarantees to
  run after every later step in the job — including your test step — even if
  an earlier step failed. (This action runs as a `node20` JS action rather
  than a composite action specifically so this teardown step can be a true
  post-job hook; composite actions have no equivalent mechanism and would run
  all their steps inline, stopping mockly immediately instead of at the end
  of the job.)

---

## GitLab CI

Include the template and extend the `.mockly-start` job:

```yaml
include:
  - remote: 'https://raw.githubusercontent.com/dever-labs/mockly/main/.gitlab/mockly.yml'

integration-tests:
  extends: .mockly-start
  variables:
    MOCKLY_VERSION: "v0.15.0" # x-release-please-version
    MOCKLY_CONFIG: "mockly.yaml"
  script:
    - ./run-tests.sh
```

Or run it as a Docker service (no binary install needed):

```yaml
integration-tests:
  image: alpine:3.21
  services:
    - name: ghcr.io/dever-labs/mockly:latest
      alias: mockly
      variables:
        # mount config via CI artifacts or inline
  variables:
    MOCKLY_URL: http://mockly:9091
  script:
    - apk add --no-cache curl
    - curl "$MOCKLY_URL/api/protocols"
    - ./run-tests.sh
```

---

## Any CI (install script)

```sh
# Install latest release
curl -sSfL https://raw.githubusercontent.com/dever-labs/mockly/main/install.sh | bash

# Or pin to a version
MOCKLY_VERSION=v0.15.0 # x-release-please-version
  curl -sSfL https://raw.githubusercontent.com/dever-labs/mockly/main/install.sh | bash

# Start in background and wait for ready
mockly start -c mockly.yaml &
until curl -sf http://localhost:9091/api/protocols; do sleep 1; done
```

Windows (PowerShell):

```powershell
# Install latest release
irm https://raw.githubusercontent.com/dever-labs/mockly/main/install.ps1 | iex

# Or pin to a version
$env:MOCKLY_VERSION = "v0.15.0" # x-release-please-version
irm https://raw.githubusercontent.com/dever-labs/mockly/main/install.ps1 | iex
```

---

## Linux packages (.deb/.rpm)

Every release publishes `.deb` and `.rpm` packages (amd64 and arm64) as release
assets, for fleets or Dockerfile/provisioning scripts managed through
`apt`/`yum` rather than a raw binary download. There's no hosted apt/yum
repository (yet) — download the asset for your release and architecture
directly:

```sh
# Debian/Ubuntu
curl -sSfL -o mockly.deb \
  https://github.com/dever-labs/mockly/releases/latest/download/mockly-linux-amd64.deb
sudo apt install ./mockly.deb

# RHEL/Fedora/CentOS
curl -sSfL -o mockly.rpm \
  https://github.com/dever-labs/mockly/releases/latest/download/mockly-linux-amd64.rpm
sudo yum install ./mockly.rpm
```

Both packages install the `mockly` binary to `/usr/bin/mockly`, so it's
immediately on `PATH`.

---

## Homebrew (macOS/Linux)

```sh
brew install dever-labs/tap/mockly
```

Published via the [`dever-labs/homebrew-tap`](https://github.com/dever-labs/homebrew-tap)
repository, updated automatically by goreleaser on every release (darwin/linux,
amd64/arm64).

---

## npm / npx

```sh
npx @dever-labs/mockly-cli start --config mockly.yaml
```

Downloads the matching platform binary on first run (no separate install
step) via [`@dever-labs/mockly-cli`](https://www.npmjs.com/package/@dever-labs/mockly-cli) —
useful in JS/TS-only CI pipelines that already have Node.js available and
would rather not shell out to curl/Docker/apt. Supports the same
`MOCKLY_DOWNLOAD_BASE_URL`/`MOCKLY_BINARY_PATH`/proxy environment variables
as [`@dever-labs/mockly-driver`](clients/node.md).

---

## Docker

```sh
# Run with your local config
docker run --rm \
  -v "$PWD/mockly.yaml:/config/mockly.yaml:ro" \
  -p 8080:8080 -p 9091:9091 \
  ghcr.io/dever-labs/mockly:latest

# Or with docker compose
docker compose up
```

---
