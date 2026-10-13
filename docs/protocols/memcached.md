## Memcached

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
