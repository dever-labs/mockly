# LDAP

LDAP mock server handling Bind (success) and Search requests. Matches on base DN and filter, then returns configured attributes.

```yaml
protocols:
  ldap:
    enabled: true
    port: 3893
    # Optional: only "v3" (the default) is currently implemented. LDAPv2
    # is a legacy, largely-obsolete dialect and is not yet supported —
    # setting "v2" fails config validation.
    protocol_version: "v3"
    mocks:
      - id: user-lookup
        base_dn: "dc=example,dc=com"
        filter: "*"
        attributes:
          cn:
            - "Alice Smith"
          mail:
            - "alice@example.com"
          uid:
            - "alice"
    # Optional: when omitted, Bind always succeeds (today's behavior,
    # unchanged) and Search returns all mocks matching the base DN/filter.
    # When present, Bind requires a matching username/password — a mismatch
    # returns invalidCredentials and Search is rejected with
    # insufficientAccessRights until a successful Bind. `allowed_mock_ids`
    # further restricts a user's Search results to the listed mock `id`s;
    # omit it (or leave it empty) to let that user see every mock.
    users:
      - username: alice
        password: secret123
        allowed_mock_ids:
          - user-lookup
```
