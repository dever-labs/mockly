# Mockly

**Cross-platform, multi-protocol mock server** — HTTP, WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, MQTT, NATS, SNMP, DNS, AMQP, Kafka, LDAP, IMAP, FTP, Memcached, STOMP, CoAP, and SIP in a single binary, with a built-in web UI, REST management API, scenario system, and fault injection.

[![CI](https://github.com/dever-labs/mockly/actions/workflows/ci.yml/badge.svg)](https://github.com/dever-labs/mockly/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/dever-labs/mockly)](https://github.com/dever-labs/mockly/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/dever-labs/mockly/blob/main/LICENSE)

[:octicons-mark-github-16: View on GitHub](https://github.com/dever-labs/mockly){ .md-button .md-button--primary }
[Quickstart](getting-started/quickstart.md){ .md-button }

## Why Mockly?

Testing against real third-party services (payment gateways, auth providers,
message brokers) is slow, flaky, and often impossible to drive into failure
scenarios on demand. Mockly runs as a single, dependency-free binary that
speaks 20+ protocols natively — not just HTTP — so you can mock the *entire*
integration surface of a modern service, not just its REST API.

- **One binary, every protocol** — HTTP, gRPC, GraphQL, WebSocket, and 16 more, all driven by the same YAML config and management API.
- **Drop-in presets** — Keycloak, Stripe, OpenAI, GitHub, Nets/Nexi payments, and a dozen others, ready to run with one command.
- **Programmable from your test suite** — official client libraries for Go, Node.js, Java, .NET, Python, and Rust, plus Testcontainers modules for Docker-based setups.
- **Built for failure testing** — scenarios and per-protocol fault injection let you simulate timeouts, errors, and edge cases without touching your hand-written mocks.

## Quick taste

```sh
# Install (see Quickstart for all options)
git clone https://github.com/dever-labs/mockly && cd mockly && make build

# Run a preset instantly
mockly preset use keycloak

# ...or start from your own config
mockly start --config mockly.yaml
```

```yaml
# mockly.yaml
mockly:
  http:
    port: 8080
    mocks:
      - id: get-user
        request:
          method: GET
          path: /users/{id}
        response:
          status: 200
          body: '{"id":"{{.request.params.id}}","name":"Alice"}'
```

[→ Full quickstart](getting-started/quickstart.md) · [→ Configuration guide](getting-started/configuration.md) · [→ All protocols](protocols/index.md)

## Features at a glance

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
| **Spec-driven generation** | `mockly generate <spec>` turns an OpenAPI, AsyncAPI, or Protobuf spec into a ready-to-run config |
| **Web UI + Management API** | Served from the binary itself — 60+ REST endpoints covering every protocol, scenario, fault, state, log, and webhook |
| **Preset configs** | Drop-in YAML for Keycloak, Authelia, OAuth2, GitHub, Stripe, OpenAI, Slack, Twilio, SendGrid, Anthropic, Resend, PagerDuty, AWS S3, NTLM, Nets/Nexi |
| **CI-friendly** | Zero dependencies, single binary, YAML config, Docker image |

See the [full feature reference](getting-started/configuration.md) for details on every matcher and response option.

## Where to go next

- **New to Mockly?** Start with the [Quickstart](getting-started/quickstart.md).
- **Configuring mocks?** See [Configuration](getting-started/configuration.md) and pick your [protocol](protocols/index.md).
- **Writing component tests?** See [Component Testing](testing/component-testing.md) and the [client libraries](clients/index.md).
- **Need a ready-made mock set?** Browse the [preset configs](presets.md).
- **Driving failures on purpose?** See [Scenarios](testing/scenarios.md) and [Fault Injection](testing/fault-injection.md).
- **Automating in CI?** See [CI Integration](ci-integration.md).
