# Memcached

Memcached text-protocol mock handling `get`, `set`, `delete`, `flush_all`, `stats`, and `quit`. Keys support `*` wildcards and `re:` regex patterns.

```yaml
protocols:
  memcached:
    enabled: true
    port: 11211
    mocks:
      - id: session-cache
        command: get
        key: "session:*"
        response:
          value: '{"user_id":42,"role":"admin"}'
      - id: any-delete
        command: delete
        key: "*"
        response:
          status: DELETED
```

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `error_type`, `message`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#http-mocks) — mocks for this protocol follow the same CRUD pattern as HTTP mocks (`/api/mocks/{protocol}`)
