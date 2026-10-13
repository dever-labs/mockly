# SMTP

SMTP server that captures emails and applies accept/reject rules.

```yaml
protocols:
  smtp:
    enabled: true
    port: 2525
    domain: mockly.local
    rules:
      - id: reject-spam
        from: "*@spam.example.com"
        action: reject
        message: "550 spam not accepted"
      - id: accept-all
        action: accept
```

Captured emails are visible at `GET /api/emails`.

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `code`, `message`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#email-inbox-smtp) — list/clear captured emails
