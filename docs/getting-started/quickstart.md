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

### Windows: Scoop

```powershell
scoop bucket add dever-labs https://github.com/dever-labs/scoop-bucket
scoop install mockly
```

This installs from the [`dever-labs/scoop-bucket`](https://github.com/dever-labs/scoop-bucket)
repository, which is updated automatically on every release. See
[CI integration](../ci-integration.md#any-ci-install-script) for the `install.ps1` alternative.

!!! note "winget support: not yet available"
    A `winget install mockly` package is planned but not live yet — goreleaser
    already generates a valid manifest on every release, but publishing it to
    [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs) requires
    a one-time manual submission first. See
    [issue #381](https://github.com/dever-labs/mockly/issues/381) for status.
    Use Scoop or the `install.ps1` script above in the meantime.

### Node.js / npm: npx

```sh
npx @dever-labs/mockly-cli start --config mockly.yaml
```

No install step needed — the first run downloads the matching platform binary automatically.
See [`@dever-labs/mockly-cli`](https://www.npmjs.com/package/@dever-labs/mockly-cli) for details,
or [the Node.js client docs](../clients/node.md) if you want to drive Mockly programmatically
from a test suite instead.

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
