# IMAP

IMAP4rev1 mock server serving pre-configured mailboxes and messages. Supports LOGIN, SELECT, FETCH, SEARCH, and LOGOUT.

```yaml
protocols:
  imap:
    enabled: true
    port: 1143
    mailboxes:
      - id: inbox
        name: INBOX
        messages:
          - seq_num: 1
            from: "sender@example.com"
            to: "user@example.com"
            subject: "Test email"
            body: "Hello world"
```

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `response`, `message`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#http-mocks) — mocks for this protocol follow the same CRUD pattern as HTTP mocks (`/api/mocks/{protocol}`)
