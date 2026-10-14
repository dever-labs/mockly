# FTP

FTP mock server with PASV support plus LIST, RETR, STOR, and DELE. Files are pre-loaded from config.

```yaml
protocols:
  ftp:
    enabled: true
    port: 2121
    passive_port_start: 50000
    files:
      - id: daily-report
        path: /reports/daily.csv
        content: |
          date,revenue
          2024-01-01,1000
      - id: app-config
        path: /data/config.json
        content: '{"version":"1.0"}'
    # Optional: when omitted, PASS always succeeds (today's behavior,
    # unchanged) and file commands (LIST/NLST/RETR/STOR/DELE/SIZE) work
    # without logging in. When present, PASS requires a matching
    # username/password — a mismatch returns "530 Login incorrect" and file
    # commands return "530" until a successful login. `allowed_files`
    # further restricts a user to the listed file `id`s; omit it (or leave
    # it empty) to let that user access every file.
    users:
      - username: alice
        password: secret123
        allowed_files:
          - daily-report
```

`passive_port_start` (default `50000`) controls the port range used for PASV
data connections: Mockly binds the first free port in a 1000-port window
starting there, instead of letting the OS pick a random ephemeral port. Set
this if you need to open a fixed, predictable range through a firewall or
NAT for FTP clients connecting to Mockly in passive mode.

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `code`, `message`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#http-mocks) — mocks for this protocol follow the same CRUD pattern as HTTP mocks (`/api/mocks/{protocol}`)
