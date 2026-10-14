# CI Integration

Mockly is a single static binary with no runtime dependencies — ideal for CI.

## GitHub Actions (composite action)

```yaml
steps:
  - uses: actions/checkout@v5

  - name: Start Mockly
    uses: dever-labs/mockly/.github/actions/setup-mockly@v0.15.0 # x-release-please-version
    with:
      version: v0.15.0         # x-release-please-version
      config: mockly.yaml      # path to your config
      api-port: 9091           # management API port (default)

  - name: Run tests
    run: npm test
```

The action automatically:
- Downloads the right binary for the runner OS/arch
- Starts mockly in the background
- Waits up to 30 s for the server to be ready
- Kills the process after the job completes

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
