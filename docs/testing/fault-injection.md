# Fault Injection

Inject protocol-native faults to test your application's resilience without touching individual mocks. Each protocol has its own fault shape using native error codes.

## Via CLI

The `mockly fault` CLI subcommand only controls **HTTP** direct fault
injection (`/api/fault/http`). For all other protocols (DNS, gRPC, Redis,
Kafka, etc.), use the [Management API](#via-api) below.

```sh
# Add 500ms latency to every HTTP request
mockly fault set --delay 500ms

# Add jittery 150ms-400ms latency to every HTTP request
mockly fault set --delay-min 150ms --delay-max 400ms

# Return 503 for every HTTP request
mockly fault set --status 503 --body '{"error":"service_unavailable"}'

# Return 429 for 30% of HTTP requests
mockly fault set --status 429 --error-rate 0.3

# Return 429 once more than 5 requests/sec are received
mockly fault set --rate-limit 5

# Show the current global fault configuration
mockly fault status

# Clear all faults
mockly fault clear
```

## Via API

```sh
# Set DNS fault
curl -X POST http://localhost:9091/api/fault/dns \
  -H 'Content-Type: application/json' \
  -d '{"rcode":"NXDOMAIN","error_rate":0.5}'

# Set gRPC fault
curl -X POST http://localhost:9091/api/fault/grpc \
  -H 'Content-Type: application/json' \
  -d '{"code":"UNAVAILABLE","delay":"500ms","error_rate":1.0}'

# Get all active faults
curl http://localhost:9091/api/fault

# Clear DNS fault only
curl -X DELETE http://localhost:9091/api/fault/dns

# Clear all faults
curl -X DELETE http://localhost:9091/api/fault

# Get the effective fault for a protocol (direct fault merged with any
# fault activated via a scenario — what will actually be applied next)
curl http://localhost:9091/api/fault/dns/effective
```

## Fault fields per protocol

| Protocol | Fields | Values / notes |
|---|---|---|
| `http` | `status`, `body`, `delay`, `delay_range`, `error_rate`, `rate_limit` | HTTP status code (default 503); `delay_range: {min, max}` jitters the delay instead of a fixed value; `rate_limit: {requests_per_second, over_limit_status, body}` returns `over_limit_status` (default 429) once the trailing 1s window is exceeded |
| `graphql` | `status`, `body`, `delay`, `error_rate` | HTTP status code (default 503) |
| `websocket` | `close_code`, `message`, `delay`, `error_rate` | WS close code (default 1011) |
| `grpc` | `code`, `message`, `delay`, `error_rate` | `UNAVAILABLE` \| `NOT_FOUND` \| `DEADLINE_EXCEEDED` \| `PERMISSION_DENIED` \| `RESOURCE_EXHAUSTED` \| `INTERNAL` |
| `tcp` | `response`, `delay`, `error_rate` | Send `response` bytes then close (default: just close) |
| `redis` | `error`, `delay`, `error_rate` | Raw Redis error string e.g. `"LOADING"` (default `"ERR fault injected"`) |
| `dns` | `rcode`, `delay`, `error_rate` | `NXDOMAIN` \| `SERVFAIL` \| `REFUSED` \| `NOTIMP` \| `FORMERR` (default `SERVFAIL`) |
| `smtp` | `code`, `message`, `delay`, `error_rate` | SMTP code e.g. 421, 450, 550 (default 421) |
| `imap` | `response`, `message`, `delay`, `error_rate` | `NO` \| `BAD` \| `BYE` (default `NO`) |
| `ftp` | `code`, `message`, `delay`, `error_rate` | FTP code e.g. 421, 530, 550 (default 421) |
| `ldap` | `result_code`, `message`, `delay`, `error_rate` | LDAP result code: 32=NO_SUCH_OBJECT, 49=INVALID_CREDENTIALS, 50=INSUFFICIENT_ACCESS, 52=UNAVAILABLE (default 52) |
| `kafka` | `error_code`, `delay`, `error_rate` | Kafka error code: 3=UNKNOWN_TOPIC, 5=LEADER_NOT_AVAILABLE, 7=REQUEST_TIMED_OUT (default 5) |
| `memcached` | `error_type`, `message`, `delay`, `error_rate` | `SERVER_ERROR` \| `CLIENT_ERROR` (default `SERVER_ERROR`) |
| `stomp` | `message`, `delay`, `error_rate` | Sends STOMP ERROR frame |
| `amqp` | `delay`, `error_rate` | Silently drops message delivery |
| `mqtt` | `delay`, `error_rate` | Silently drops response publish |
| `nats` | `delay`, `error_rate` | Silently drops mock response/reply |
| `coap` | `code`, `delay`, `error_rate` | CoAP code: `4.01`, `4.03`, `4.04`, `5.00`, `5.03` (default `5.00`) |
| `sip` | `status`, `reason`, `delay`, `error_rate` | SIP status: 404, 408, 486, 503 (default 503) |
| `snmp` | `message`, `delay`, `error_rate` | Returns error from OID callback |

`error_rate`: probability 0.0–1.0 that the fault fires; 0 = always (default).

## Via scenarios (recommended for reproducible tests)

See [Fault injection in scenarios](scenarios.md#fault-injection-in-scenarios).

---
