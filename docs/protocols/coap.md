## CoAP

CoAP UDP mock server handling GET, POST, PUT, and DELETE requests. Matches on method + path with exact, wildcard, named segment, or regex patterns. Use `path_regex` when you want a dedicated regex field.

```yaml
protocols:
  coap:
    enabled: true
    port: 5683
    mocks:
      - id: temperature
        method: GET
        path: /sensors/temperature
        response:
          code: "2.05"
          payload: "23.5"
          content_format: 0   # text/plain

      - id: region-sensor
        method: GET
        path: /sensors/{type}
        response:
          code: "2.05"
          payload: "Reading for {{.request.params.type}}: 42"

      - id: sensor-regex
        method: GET
        path_regex: "^/sensors/[a-z]+$"   # alternative regex field
        response:
          code: "2.05"
          payload: "ok"
```
