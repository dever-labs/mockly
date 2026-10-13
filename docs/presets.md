# Preset Configs

Mockly ships with pre-built YAML configs for common services:

| Preset | Description |
|---|---|
| `keycloak` | Token endpoint, JWKS, userinfo, introspection |
| `authelia` | Auth verify, session endpoints |
| `oauth2` | Generic OAuth2 flows (authorize, token, revoke) |
| `ntlm` | Windows NTLM authentication — full 3-step handshake example |
| `github` | REST API: repos, issues, pull requests |
| `stripe` | Charges, refunds, customers, payment intents |
| `openai` | Chat completions, embeddings, models |
| `slack` | Messages, channels, users, reactions |
| `twilio` | SMS, calls, lookup |
| `sendgrid` | Email send, templates, contacts |
| `anthropic` | Claude messages, models |
| `resend` | Email send, retrieve, domains, API keys |
| `pagerduty` | Incidents, services, users, escalations |
| `aws-s3` | List buckets/objects, get/put/delete objects |
| `nets` | Nets/Nexi Easy Checkout payments — create/get/charge/refund/cancel, an outbound "payment created" webhook, plus a hosted checkout page worked example (`body_form` matching) |

Each preset also includes built-in scenarios for common failure modes (e.g. `keycloak-unauthorized`, `stripe-card-declined`).

## Use a preset

```sh
mockly preset use keycloak
```

## Import a preset into your own config

```sh
mockly preset show keycloak > keycloak.yaml
# Edit keycloak.yaml, then:
mockly start --config keycloak.yaml
```

---
