<p align="center">
  <img src="docs/assets/logo.svg" alt="Mockly logo" width="88" height="auto">
</p>

<h1 align="center">Mockly</h1>

<p align="center">
  <strong>One binary. Twenty protocols. Every failure mode your real dependencies won't let you test.</strong>
</p>

<p align="center">
  <a href="https://github.com/dever-labs/mockly/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/dever-labs/mockly/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/dever-labs/mockly/releases/latest"><img alt="Latest release" src="https://img.shields.io/github/v/release/dever-labs/mockly"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-blue.svg"></a>
</p>

## Why Mockly?

Testing against real third-party services — payment gateways, auth providers, message brokers — is slow, flaky, and often impossible to drive into the failure scenarios you actually need to cover. Mockly is a single, dependency-free binary that speaks **HTTP, WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, MQTT, NATS, SNMP, DNS, AMQP, Kafka, LDAP, IMAP, FTP, Memcached, STOMP, CoAP, and SIP** natively, with a built-in web UI, a REST management API, scenarios, and fault injection — so you can mock the *entire* integration surface of a modern service, not just its REST API, and reliably trigger the edge cases real vendors won't.

## Features

| Feature | Details |
|---|---|
| **Protocols** | HTTP, WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, MQTT, NATS, SNMP, DNS, AMQP, Kafka, LDAP, IMAP, FTP, Memcached, STOMP, CoAP, SIP |
| **Request matching** | Method + path (exact / wildcard / named params / regex), headers (with `re:` pattern support), query params (exact / wildcard / regex / absence / repeated-value), JSON/form/multipart/XML body fields, authentication |
| **Response sequences** | Return a different response on each successive call — loop, hold last, or 404 when exhausted |
| **Template responses** | Go template syntax in response bodies and headers (`{{now}}`, `{{.request.params.id}}`, `{{.request.body.foo}}`, `{{uuid}}`, `{{fake "email"}}`, etc.) |
| **Scenarios** | Named sets of mock patches — activate/deactivate atomically via API or CLI |
| **Fault injection** | Per-protocol native fault fields (DNS rcode, gRPC status code, Kafka error code, etc.) and per-mock faults (delay, error rate, rate limiting) |
| **Call verification** | Track how many times each mock was hit; block until an expected count is reached |
| **Record mode (HTTP)** | Proxy unmatched requests to a real upstream and capture the response as a new mock — bootstrap a mock set without hand-authoring every response |
| **Outbound webhooks** | Any HTTP mock can fire a templated outbound callback when matched, with delay, retries, and a searchable attempt history |
| **PATCH mocks** | Change only specific response fields at runtime without replacing the whole mock |
| **Preset configs** | Drop-in YAML for Keycloak, Authelia, OAuth2, GitHub, Stripe, OpenAI, Slack, Twilio, SendGrid, Anthropic, Resend, PagerDuty, AWS S3, NTLM, Nets/Nexi |
| **Spec-driven generation** | `mockly generate <spec>` turns an OpenAPI, AsyncAPI, or Protobuf spec into a ready-to-run config |
| **Web UI + Management API** | Served from the binary itself — 60+ REST endpoints covering every protocol, scenario, fault, state, log, and webhook |
| **Observability** | Opt-in Prometheus `/metrics` endpoint |
| **CI-friendly** | Zero dependencies, single binary, YAML config, Docker image |

See [all features and matchers](docs/getting-started/configuration.md) for full details.

## Quickstart

```sh
git clone https://github.com/dever-labs/mockly && cd mockly && make build
mockly preset use keycloak    # starts Mockly pre-loaded with Keycloak endpoints
# ...or:
mockly start --config mockly.yaml
```

Open `http://localhost:9091` for the web UI, or call the management API at the same port.

**→ [Full quickstart](docs/getting-started/quickstart.md) · [Configuration guide](docs/getting-started/configuration.md)**

## Documentation

| Topic | Link |
|---|---|
| Getting started & configuration | [docs/getting-started/](docs/getting-started/) |
| All 20 protocols (HTTP, gRPC, GraphQL, Kafka, SNMP, ...) | [docs/protocols/](docs/protocols/) |
| Component testing, scenarios, fault injection | [docs/testing/](docs/testing/) |
| Preset configs | [docs/presets.md](docs/presets.md) |
| CLI & Management API reference | [docs/reference/](docs/reference/) |
| Client libraries (Go, Node, Java, .NET, Python, Rust) | [docs/clients/](docs/clients/) |
| CI integration (GitHub Actions, GitLab CI, Docker) | [docs/ci-integration.md](docs/ci-integration.md) |
| Architecture | [docs/architecture.md](docs/architecture.md) |
| Development & contributing | [docs/contributing.md](docs/contributing.md) |

All of the above is also published, with search and navigation, at **[mockly.dever.dk](https://mockly.dever.dk)**.

## Client Libraries

Mockly ships official clients for both native-process and Docker-backed test setups, so you can choose between a locally managed binary or a containerized Mockly instance.

| Language | Driver package | Testcontainers package |
|---|---|---|
| **Go** | `github.com/dever-labs/mockly/clients/go` | `github.com/dever-labs/mockly/clients/go/testcontainers` |
| **Node.js / TypeScript** | `@dever-labs/mockly-driver` | `@dever-labs/mockly-testcontainers` |
| **Java** | `io.github.dever-labs:mockly-driver` | `io.github.dever-labs:mockly-testcontainers` |
| **.NET / C#** | `Mockly.Driver` | `Testcontainers.Mockly` |
| **Python** | `mockly-driver` | `mockly-testcontainers` |
| **Rust** | `mockly-driver` | `mockly-testcontainers` |

**→ [Full client library docs](docs/clients/index.md)**

## Development

```sh
make build        # build UI + Go binary
make test         # run unit + integration tests
make test-e2e     # run e2e tests (builds binary first)
make lint         # run golangci-lint
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for full setup instructions, commit conventions, and the PR process.

## Contributing

Contributions are welcome — bug reports, feature requests, preset configs, and code.

Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request. By participating you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

For security issues, follow the process in [SECURITY.md](SECURITY.md) — **do not open a public issue**.

## License

Copyright © 2026 dever-labs. Released under the [MIT License](LICENSE).
