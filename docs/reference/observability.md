# Observability

## Prometheus metrics

Enable an opt-in `GET /metrics` endpoint (Prometheus text exposition format)
on the management API for scraping request-rate, latency, and error-rate
metrics into Grafana/Alertmanager or any Prometheus-compatible stack — handy
when Mockly runs as a long-lived shared mock service in CI.

```yaml
mockly:
  api:
    port: 9091
    metrics:
      enabled: true   # disabled by default
```

```sh
curl http://localhost:9091/metrics
```

| Metric | Type | Labels | Description |
|---|---|---|---|
| `mockly_http_requests_total` | counter | `mock_id`, `method`, `status` | Total HTTP mock requests handled. Unmatched requests are labeled `mock_id="unmatched"`. |
| `mockly_http_request_duration_seconds` | histogram | `mock_id` | HTTP mock request handling duration, in seconds. |
| `mockly_active_mocks` | gauge | — | Number of currently configured HTTP mocks. |

---
