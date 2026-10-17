# Quickstart

## Download

Grab the binary for your platform from the [releases page](https://github.com/dever-labs/mockly/releases), or build from source:

```sh
git clone https://github.com/dever-labs/mockly
cd mockly
make build        # builds UI + Go binary
```

### Linux: apt / yum

Every release also publishes `.deb` and `.rpm` packages as release assets (see
[CI integration](../ci-integration.md#linux-packages-debrpm) for the exact
install commands) — handy for fleets or Dockerfile/provisioning scripts that
manage tooling through `apt`/`yum` rather than a raw binary download.

### macOS / Linux: Homebrew

```sh
brew install dever-labs/tap/mockly
```

This installs from the [`dever-labs/homebrew-tap`](https://github.com/dever-labs/homebrew-tap)
repository, which is updated automatically on every release.

## Run with a config file

```sh
mockly start --config mockly.yaml
```

Open `http://localhost:9091` for the web UI, or call the management API at the same port.

## Run a preset

```sh
mockly preset use keycloak    # starts Mockly pre-loaded with Keycloak endpoints
mockly preset list            # list all available presets
mockly preset show stripe     # print the preset YAML
```

---
