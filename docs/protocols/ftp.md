# FTP

FTP mock server with PASV support plus LIST, RETR, STOR, and DELE. Files are pre-loaded from config.

```yaml
protocols:
  ftp:
    enabled: true
    port: 2121
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
