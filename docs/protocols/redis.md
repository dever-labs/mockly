# Redis

RESP-protocol Redis mock. Intercepts any Redis command and returns a configurable response.

```yaml
protocols:
  redis:
    enabled: true
    port: 6379
    mocks:
      - id: auth
        command: AUTH
        response:
          type: string     # string | bulk | integer | null | error | array
          value: "OK"
      - id: get-token
        command: GET
        key: "token:*"
        response:
          type: bulk
          value: "abc123"
          delay: 5ms
```

## Stateful mode

By default the Redis mock only matches static mocks — `SET`/`GET` don't actually round-trip data. Set `mode: stateful` to enable a real in-memory datastore:

```yaml
protocols:
  redis:
    enabled: true
    port: 6379
    mode: stateful
    mocks: []   # still used as a fallback for commands not listed below
```

In stateful mode these commands are backed by a real per-connection-shared datastore with Redis-compatible semantics (including `WRONGTYPE` errors when a key holds the wrong type):

- **Strings**: `SET` (with optional `EX seconds` / `PX milliseconds`), `GET`, `APPEND`, `INCR`, `DECR`, `INCRBY`, `DECRBY`
- **Keys**: `DEL`, `EXISTS`, `EXPIRE`, `TTL`, `PERSIST`
- **Hashes**: `HSET`, `HGET`, `HGETALL`, `HDEL`, `HEXISTS`
- **Lists**: `LPUSH`, `RPUSH`, `LRANGE`, `LLEN`

`FLUSHDB`/`FLUSHALL` clear the stateful datastore too, and `POST /api/reset` wipes it along with the rest of mock state. Any command not in the list above (e.g. `TYPE`, `SCAN`) still falls back to the static mocks configured for that server.

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `error`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#http-mocks) — mocks for this protocol follow the same CRUD pattern as HTTP mocks (`/api/mocks/{protocol}`)
