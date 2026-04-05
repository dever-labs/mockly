# Contributing to Mockly

Thank you for your interest in contributing! This document explains how to get
started, the development workflow, and what we expect from contributions.

## Table of contents

- [Code of conduct](#code-of-conduct)
- [Ways to contribute](#ways-to-contribute)
- [Getting started](#getting-started)
- [Development workflow](#development-workflow)
- [Commit messages](#commit-messages)
- [Pull request process](#pull-request-process)
- [Adding a new protocol](#adding-a-new-protocol)
- [Adding a preset config](#adding-a-preset-config)

---

## Code of conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
By participating you agree to abide by its terms.

## Ways to contribute

- **Bug reports** — open an issue with the `bug` label; include the Mockly
  version, OS, a minimal `mockly.yaml`, and the exact error
- **Feature requests** — open an issue with the `enhancement` label; describe
  the use case, not just the solution
- **Preset configs** — YAML files under `configs/` for common services are very
  welcome; see [Adding a preset config](#adding-a-preset-config)
- **Code** — bug fixes, new features, improved test coverage, documentation

## Getting started

### Prerequisites

| Tool | Minimum version |
| ---- | --------------- |
| Go   | 1.23            |
| Node | 20              |
| npm  | 9               |
| make | any             |

### Local setup

```bash
git clone https://github.com/dever-labs/mockly.git
cd mockly

# Install Go dependencies
go mod download

# Install UI dependencies and build the embedded UI
cd ui && npm install && npm run build && cd ..

# Run the server
go run ./cmd/mockly start
```

### Running tests

```bash
# Unit + integration tests
make test

# E2E tests (builds the binary first)
make test-e2e
```

## Development workflow

1. Fork the repo and create a branch from `main`:
   ```bash
   git checkout -b fix/my-bug-fix
   ```
2. Make your changes — keep them focused; one concern per PR
3. Add or update tests to cover your change
4. Run `make test` and `make test-e2e` — both must pass
5. Push and open a pull request against `main`

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

[optional body]
```

Common types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `ci`

Examples:
```
feat(http): support response templating with Go text/template
fix(reset): restore mocks to config defaults on POST /api/reset
docs: add gRPC preset example to README
```

Commits that land on `main` are squash-merged so history stays clean.

## Pull request process

1. Fill in the PR template
2. Ensure CI is green (unit tests, E2E, UI build, binary build)
3. A maintainer will review within a few business days
4. Address review comments; force-push to the same branch
5. Once approved, a maintainer will squash-merge

## Adding a new protocol

1. Create `internal/protocols/<name>server/server.go` implementing the
   `engine.Protocol` interface
2. Add the config struct to `internal/config/config.go` and `ProtocolsConfig`
3. Wire it up in `cmd/mockly/main.go` and `internal/api/server.go`
4. Add management API routes in `internal/api/server.go` (CRUD for mocks)
5. Write unit tests in `internal/protocols/<name>server/server_test.go`
6. Update `README.md` with configuration reference

## Adding a preset config

Preset configs live under `configs/<service>/`. A preset is a ready-to-use
`mockly.yaml` (or partial YAML) that mocks a well-known service:

```
configs/
  keycloak/
    mockly.yaml       # full config with common endpoints
    README.md         # short explanation of what's mocked
  authelia/
    mockly.yaml
    README.md
```

Keep presets realistic: use real-world paths, realistic response shapes, and
document any assumptions (e.g. realm name, client ID).
