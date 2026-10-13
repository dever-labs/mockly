# DNS

Full DNS mock server over UDP and TCP. Responds to `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `PTR`, `SRV`, and `NS` queries. `records` hold the raw answer values; for `MX` use `"<priority> <host>"`, and for `SRV` use `"<priority> <weight> <port> <target>"`.

```yaml
protocols:
  dns:
    enabled: true
    port: 5353
    mocks:
      - id: api-host
        name: "api.example.com"
        type: A
        records:
          - "127.0.0.1"
        ttl: 60
      - id: mail
        name: "example.com"
        type: MX
        records:
          - "10 mail.example.com"
```
