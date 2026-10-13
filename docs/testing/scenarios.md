# Scenarios

Scenarios let you pre-define named mock overrides and activate/deactivate them at any time — great for toggling between happy path and failure modes during testing.

## Define in config

```yaml
scenarios:
  - id: payment-timeout
    name: Payment Gateway Timeout
    patches:
      - mock_id: charge
        status: 504
        body: '{"error":"timeout"}'
        delay: 5s
      - mock_id: refund
        disabled: true   # Removes this endpoint entirely
```

## Control via CLI

```sh
mockly scenario list
mockly scenario activate payment-timeout
mockly scenario deactivate payment-timeout
```

## Control via API

```sh
# Activate
curl -X POST http://localhost:9091/api/scenarios/payment-timeout/activate

# Deactivate
curl -X DELETE http://localhost:9091/api/scenarios/payment-timeout/activate

# List active
curl http://localhost:9091/api/scenarios/active
```

## Fault injection in scenarios

Scenarios can bundle protocol faults alongside mock patches — activating a scenario sets both atomically:

```yaml
scenarios:
  - id: backend-degraded
    name: Backend Degraded
    patches:
      - mock_id: get-user
        status: 503
    faults:
      redis:
        error: "LOADING Redis is loading the dataset in memory"
        error_rate: 1.0
      grpc:
        code: UNAVAILABLE
        delay: 500ms
        error_rate: 0.8

  - id: dns-failure
    name: DNS Resolution Failure
    faults:
      dns:
        rcode: NXDOMAIN
        error_rate: 0.5
```

When `backend-degraded` is activated: the `get-user` mock is patched and Redis/gRPC start returning faults. Deactivating the scenario restores normal behaviour.

---
