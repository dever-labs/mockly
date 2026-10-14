# Outbound Webhooks

Any HTTP mock can fire a templated outbound HTTP callback when it's matched
— simulating systems that notify clients asynchronously via server-initiated
requests (payment gateway callbacks, job-completion notifications, message
broker pushes, etc.) rather than only responding synchronously.

Dispatch is fire-and-forget: the callback is sent in the background and
never blocks or delays the mock's own HTTP response. `url`, `headers`, and
`body` all support the same Go template syntax as response bodies, so the
callback target and payload can be derived from the triggering request
(e.g. a caller-supplied `callback_url` field).

```yaml
      - id: create-payment
        request:
          method: POST
          path: /v1/payment
        response:
          status: 201
          body: '{"paymentId":"pay_123"}'
        webhooks:
          - url: "{{.request.body.webhookUrl}}"
            method: POST
            headers:
              Authorization: "{{.request.body.webhookSecret}}"
            delay: 300ms        # wait before sending, simulating async processing
            retries: 2          # additional attempts on failure or a 5xx response
            retry_delay: 1s
            body: |
              {"event": "payment.created", "id": "{{uuid}}"}
```

Every attempt (including retries) is recorded and available via the
management API:

```sh
curl http://localhost:9091/api/webhooks                 # attempt history
curl -X DELETE http://localhost:9091/api/webhooks        # clear history
curl -X POST http://localhost:9091/api/webhooks/send \
  -d '{"url":"https://example.com/hook","body":"{\"ping\":true}"}'  # send one ad hoc
```

See the [Nets/Nexi preset](https://github.com/dever-labs/mockly/blob/main/configs/presets/nets.yaml) for a full example
that templates the callback URL and authorization straight out of the
request body, matching how real payment gateways accept a `webhooks` array.

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Management API](../reference/management-api.md#outbound-webhooks) — list/clear attempt history, fire a one-off webhook
