# TCP

Raw TCP mock server. Matches incoming data as exact string, prefix wildcard, or regex. Supports hex encoding for binary protocols.

```yaml
protocols:
  tcp:
    enabled: true
    port: 8083
    mocks:
      - id: ping
        match: "PING\r\n"
        response: "+PONG\r\n"
      - id: hex-response
        match: "re:^\\x02.*\\x03$"
        response: "hex:060000"
```

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `response`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#http-mocks) — mocks for this protocol follow the same CRUD pattern as HTTP mocks (`/api/mocks/{protocol}`)
