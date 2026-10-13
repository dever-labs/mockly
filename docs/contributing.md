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

## License

Copyright © 2026 dever-labs. Released under the [MIT License](https://github.com/dever-labs/mockly/blob/main/LICENSE).
