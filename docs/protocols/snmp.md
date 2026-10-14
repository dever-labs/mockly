# SNMP

Full SNMP agent (built on gosnmp/gosnmp) that responds to GET, GETNEXT, GETBULK, and SET requests. Supports SNMPv1, v2c, and v3 (USM with MD5/SHA auth and DES/AES privacy). Can also send outbound TRAPs to any target host via the management API.

```yaml
protocols:
  snmp:
    enabled: true
    port: 1161            # default; 161 requires root / CAP_NET_BIND_SERVICE
    community: "public"   # v1/v2c community string
    # Optional: restrict the accepted SNMP dialect(s) ("v1", "v2c", or
    # "v3"). Omit to accept any dialect (default). "v3" rejects v1/v2c
    # requests; "v1"/"v2c" disables v3/USM authentication (v1 and v2c are
    # not distinguished from each other — both remain accepted together).
    protocol_version: "v3"
    v3_users:
      - username: mocklyuser
        auth_protocol: md5        # md5 | sha | sha224 | sha256 | sha384 | sha512
        auth_passphrase: mocklyauth
        priv_protocol: des        # des | aes | aes192 | aes256
        priv_passphrase: mocklypriv
    mocks:
      - id: sys-descr
        oid: 1.3.6.1.2.1.1.1.0
        type: string
        value: "Mockly Virtual Device"
      - id: sys-uptime
        oid: 1.3.6.1.2.1.1.3.0
        type: timeticks
        value: 987654
      - id: if-number
        oid: 1.3.6.1.2.1.2.1.0
        type: integer
        value: 4
    traps:
      - id: cold-start
        target: "127.0.0.1:1162"
        version: "2c"
        community: "public"
        oid: 1.3.6.1.6.3.1.1.5.1
        bindings:
          - oid: 1.3.6.1.2.1.1.1.0
            type: string
            value: "Device restarted"
      - id: cold-start-v1
        target: "127.0.0.1:1162"
        version: "1"               # RFC 1157 Trap-PDU requires enterprise/agent_address
        community: "public"
        oid: 1.3.6.1.6.3.1.1.5.1
        enterprise: "1.3.6.1.4.1.9999"   # optional; defaults to `oid` if omitted
        agent_address: "10.0.0.5"       # optional; defaults to the agent's outbound local IP
        generic_trap: 0                 # optional; 0 = coldStart (RFC 1157 generic-trap enum)
        specific_trap: 0                # optional; used only when generic_trap = 6 (enterpriseSpecific)
```

`enterprise`, `agent_address`, `generic_trap`, and `specific_trap` only apply to `version: "1"` traps (ignored for v2c/v3).


**Supported OID types:**

| `type` value | SNMP ASN.1 type | Example `value` |
|---|---|---|
| `string` / `octetstring` | OctetString | `"Mockly Virtual Device"` |
| `integer` / `int` | Integer | `42` |
| `gauge32` | Gauge32 | `100` |
| `counter32` | Counter32 | `1048576` |
| `counter64` | Counter64 | `9000000000` |
| `timeticks` | TimeTicks | `987654` |
| `ipaddress` | IPAddress | `"192.168.1.1"` |
| `objectidentifier` / `oid` | ObjectIdentifier | `"1.3.6.1.2.1.1.2.0"` |

**TRAP sending** — POST to `/api/snmp/traps/{id}/send` to trigger any configured trap. The agent connects to the trap's `target` over UDP and sends the PDU.

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `message`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#snmp-mocks-traps) — CRUD for OID mocks and traps, send a trap on demand
