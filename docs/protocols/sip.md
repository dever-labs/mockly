# SIP

SIP UDP mock server handling INVITE, REGISTER, OPTIONS, BYE, CANCEL, and ACK. Matches on method + URI using exact, wildcard, named segment, or regex patterns. Use `uri_regex` when you want a dedicated regex field.

```yaml
protocols:
  sip:
    enabled: true
    port: 5060
    mocks:
      - id: invite-ok
        method: INVITE
        uri: "sip:*@example.com"
        response:
          status: 200
          reason: "OK"
      - id: register
        method: REGISTER
        uri: "*"
        response:
          status: 200
          reason: "OK"
      - id: any-invite
        method: INVITE
        uri_regex: "^sip:[a-z]+@example\\.com$"
        response:
          status: 200
          reason: "OK"
```

If your SIP URI includes path segments, named captures are available in response templates, for example `{{.request.params.user}}` with a pattern like `sip:gateway.example.com/users/{user}`.

---
