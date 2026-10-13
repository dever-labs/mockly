# Management API Reference

Base URL: `http://localhost:9091`

> **API documentation files** — two ready-to-use references ship in `docs/`:
>
> | File | Format | How to use |
> |---|---|---|
> | [`openapi.yaml`](../openapi.yaml) | OpenAPI 3.1 | Open in [Swagger UI](https://editor.swagger.io/), Redoc, Stoplight, or any OpenAPI-compatible tool |
> | [`mockly.postly.json`](../mockly.postly.json) | [Postly](https://github.com/dever-labs/postly) collection | Import into Postly and set the `baseUrl` environment variable to your Mockly instance (e.g. `http://localhost:9091`) |

## Protocols

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/protocols` | List all protocol statuses |
| `GET` | `/api/health` | Health check |

## HTTP Mocks

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/mocks/http` | List HTTP mocks |
| `POST` | `/api/mocks/http` | Create HTTP mock |
| `PUT` | `/api/mocks/http/{id}` | Replace HTTP mock |
| `PATCH` | `/api/mocks/http/{id}` | Partial update HTTP mock |
| `DELETE` | `/api/mocks/http/{id}` | Delete HTTP mock |

Similarly for WebSocket (`/api/mocks/websocket`), gRPC (`/api/mocks/grpc`), GraphQL (`/api/mocks/graphql`), TCP (`/api/mocks/tcp`), Redis (`/api/mocks/redis`), SMTP (`/api/mocks/smtp`), MQTT (`/api/mocks/mqtt`), NATS (`/api/mocks/nats`), SNMP (`/api/mocks/snmp`), DNS (`/api/mocks/dns`), AMQP (`/api/mocks/amqp`), Kafka (`/api/mocks/kafka`), LDAP (`/api/mocks/ldap`), IMAP (`/api/mocks/imap`), FTP (`/api/mocks/ftp`), Memcached (`/api/mocks/memcached`), STOMP (`/api/mocks/stomp`), CoAP (`/api/mocks/coap`), and SIP (`/api/mocks/sip`).

## Call Verification (HTTP)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/calls/http/{mockId}` | Get call count + log entries for a mock |
| `POST` | `/api/calls/http/{mockId}/wait` | Block until mock has been called N times (body: `{"count":N,"timeout":"5s"}`) |
| `DELETE` | `/api/calls/http/{mockId}` | Clear log entries for mock + reset all HTTP call counts |
| `DELETE` | `/api/calls/http` | Clear all HTTP log entries and reset all call counts |

## Email inbox (SMTP)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/emails` | List captured emails |
| `DELETE` | `/api/emails` | Clear inbox |

## Outbound Webhooks

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/webhooks` | List the outbound webhook attempt history |
| `DELETE` | `/api/webhooks` | Clear the webhook attempt history |
| `POST` | `/api/webhooks/send` | Fire a one-off webhook synchronously (body: `{"url":"...","method":"POST","headers":{...},"body":"..."}`); returns the resulting attempt record |

## MQTT messages

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/mqtt/messages` | List captured MQTT messages |
| `DELETE` | `/api/mqtt/messages` | Clear message store |

## NATS messages

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/nats/messages` | List captured NATS messages |
| `DELETE` | `/api/nats/messages` | Clear message store |

## Message stores (AMQP, Kafka, STOMP)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/amqp/messages` | List captured AMQP messages |
| `DELETE` | `/api/amqp/messages` | Clear AMQP message store |
| `GET` | `/api/kafka/messages` | List captured Kafka messages |
| `DELETE` | `/api/kafka/messages` | Clear Kafka message store |
| `GET` | `/api/stomp/messages` | List captured STOMP messages |
| `DELETE` | `/api/stomp/messages` | Clear STOMP message store |

## SNMP Mocks & Traps

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/mocks/snmp` | List configured OID mocks |
| `POST` | `/api/mocks/snmp` | Add an OID mock |
| `PUT` | `/api/mocks/snmp/{id}` | Replace an OID mock |
| `DELETE` | `/api/mocks/snmp/{id}` | Remove an OID mock |
| `GET` | `/api/snmp/traps` | List configured traps |
| `POST` | `/api/snmp/traps` | Add a trap config |
| `POST` | `/api/snmp/traps/{id}/send` | Send a configured trap immediately |

## Streaming Connections (HTTP `keep_open`, gRPC `keep_open`, GraphQL subscriptions)

See [Stream management API (push / close)](../protocols/graphql.md#stream-management-api-push-close) for request/response shapes.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/streams` | List every currently open streaming connection, across all protocols |
| `POST` | `/api/streams/push` | Push one more ad-hoc event into an open connection (by `conn_id` or `mock_id`) |
| `POST` | `/api/streams/close` | Close an open connection, optionally as an error |

## Scenarios

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/scenarios` | List all scenarios |
| `POST` | `/api/scenarios` | Create scenario |
| `GET` | `/api/scenarios/active` | List active scenarios |
| `GET` | `/api/scenarios/{id}` | Get scenario |
| `PUT` | `/api/scenarios/{id}` | Replace scenario |
| `DELETE` | `/api/scenarios/{id}` | Delete scenario |
| `POST` | `/api/scenarios/{id}/activate` | Activate scenario |
| `DELETE` | `/api/scenarios/{id}/activate` | Deactivate scenario |

## Fault Injection

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/fault` | Get all active direct faults |
| `DELETE` | `/api/fault` | Clear all direct faults |
| `GET` | `/api/fault/{protocol}` | Get fault config for a protocol |
| `POST` | `/api/fault/{protocol}` | Set fault config for a protocol |
| `DELETE` | `/api/fault/{protocol}` | Clear fault for a protocol |
| `GET` | `/api/fault/{protocol}/effective` | Get the effective fault (direct + active scenario) for a protocol |

`{protocol}` is one of: `http`, `graphql`, `websocket`, `grpc`, `tcp`, `redis`, `mqtt`, `smtp`, `snmp`, `dns`, `amqp`, `kafka`, `ldap`, `imap`, `ftp`, `memcached`, `stomp`, `coap`, `sip`.

## State Store

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/state` | Get all (non-expired) state keys |
| `POST` | `/api/state` | Set state keys (JSON object) |
| `POST` | `/api/state?ttl=<duration>` | Set state keys that auto-expire after `<duration>` (e.g. `30s`, `5m`); omitted = no expiry |
| `DELETE` | `/api/state/{key}` | Delete a single state key |
| `DELETE` | `/api/state?prefix=<prefix>` | Delete only keys starting with `<prefix>`, leaving others untouched; omitted/empty prefix clears all state |

State keys never persist across restarts (in-memory only). TTL expiry is
checked lazily (on read), so an expired key simply disappears the next time
it's fetched — there's no background sweep. To avoid unrelated mocks or
parallel test runs clobbering each other's state, namespace your keys by
convention (e.g. `"login:session"`, `"cart:items"`) and use the prefix-scoped
delete above to reset just one namespace instead of wiping everything.

## Logs

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/logs` | Get recent log entries (all protocols) |
| `GET` | `/api/logs?matched_id=<id>` | Filter log entries by mock ID |
| `GET` | `/api/logs/count` | Count all log entries |
| `GET` | `/api/logs/count?matched_id=<id>` | Count log entries for a specific mock |
| `DELETE` | `/api/logs` | Clear all logs |
| `GET` | `/api/logs/stream` | SSE stream of live log entries |

## Reset

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/reset` | Reset all mocks/state/logs/fault/scenarios to config defaults |

---
