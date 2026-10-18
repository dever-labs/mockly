# Development & Contributing

## Development

```sh
make build        # build UI + Go binary
make test         # run unit + integration tests
make test-e2e     # run e2e tests (builds binary first)
make lint         # run golangci-lint
make vulncheck    # scan Go dependencies for known vulnerabilities (govulncheck)
make check-coverage  # verify test coverage meets the CI threshold (run after `make test`)
make dev          # hot-reload with air
```

See [CONTRIBUTING.md](https://github.com/dever-labs/mockly/blob/main/CONTRIBUTING.md) for full setup instructions, commit conventions, and the PR process.

## Contributing

Contributions are welcome — bug reports, feature requests, preset configs, and code.

Please read [CONTRIBUTING.md](https://github.com/dever-labs/mockly/blob/main/CONTRIBUTING.md) before opening a pull request.
By participating you agree to follow the [Code of Conduct](https://github.com/dever-labs/mockly/blob/main/CODE_OF_CONDUCT.md).

For security issues, follow the process in [SECURITY.md](https://github.com/dever-labs/mockly/blob/main/SECURITY.md) — **do not open a public issue**.

## Releasing (maintainers)

Releases are built and published automatically by goreleaser (see
[`.goreleaser.yaml`](https://github.com/dever-labs/mockly/blob/main/.goreleaser.yaml))
once a version tag is pushed. Most package registries (Homebrew, Scoop, npm,
PyPI, crates.io, NuGet, Maven Central, container images, `.deb`/`.rpm`) are
fully automated and need no manual steps.

**winget is the one exception**, because publishing requires a pull request
against Microsoft's own [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs)
repository rather than a repo `dever-labs` controls outright. goreleaser's
`winget:` pipe already generates a valid manifest on every release, but ships
with `skip_upload: true` until the following **one-time, manual** bootstrap
is completed (tracked in [#381](https://github.com/dever-labs/mockly/issues/381)):

1. Fork `microsoft/winget-pkgs` into the `dever-labs` org.
2. Confirm the `TAP_GITHUB_TOKEN` org secret (or a dedicated token) has push
   access to that fork and `public_repo` scope to open pull requests against
   upstream `microsoft/winget-pkgs`.
3. Manually submit the first `dever-labs.mockly` manifest — e.g. via
   [`wingetcreate submit`](https://github.com/microsoft/winget-create) — so
   winget-pkgs' validation bot approves the `dever-labs.mockly` package
   identifier once.

Once that's done, remove `skip_upload: true` from the `winget:` pipe in
`.goreleaser.yaml` so future releases push to the fork and open a PR
automatically, the same way `homebrew_casks`/`scoops` already do.

## License

Copyright © 2026 dever-labs. Released under the [MIT License](https://github.com/dever-labs/mockly/blob/main/LICENSE).
