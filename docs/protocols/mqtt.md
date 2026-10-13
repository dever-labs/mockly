## MQTT

Full MQTT v3/v4/v5 broker (powered by mochi-mqtt). Configurable topic pattern matching with automatic response publishing.

```yaml
protocols:
  mqtt:
    enabled: true
    port: 1883
    # Optional: pin the broker to a single MQTT wire dialect ("3.1",
    # "3.1.1", or "5.0"). Omit to auto-negotiate per client (default).
    # CONNECT attempts using any other version are refused.
    protocol_version: "5.0"
    mocks:
      - id: command-ack
        topic: "devices/+/command"
        response:
          topic: "devices/+/ack"
          payload: '{"status":"ok"}'
          qos: 1
```

Topic wildcards: `+` matches a single segment, `#` matches everything below. `{name}` captures a single segment value for use in response templates and logs.

```yaml
      - id: device-command
        topic: "devices/{device_id}/command"
        response:
          topic: "devices/{{.request.params.device_id}}/ack"
          payload: '{"device":"{{.request.params.device_id}}","status":"ok"}'
```

---
