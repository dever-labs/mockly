## TCP

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
