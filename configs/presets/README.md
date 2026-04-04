# Mockly Presets

Pre-built mock configurations for commonly used APIs and services.
All presets are bundled into the `mockly` binary and available without any files on disk.

## Usage

```bash
# List all available presets
mockly preset list

# Start with a preset
mockly preset use keycloak
mockly preset use stripe --http-port 9000
mockly preset use openai --api-port 9095

# Print a preset's YAML (pipe to a file to customise it)
mockly preset show keycloak > my-keycloak.yaml

# Start from a config file (for full control)
mockly start --config configs/presets/keycloak.yaml
```

## Available Presets

| Name | Description | HTTP Port | Key Endpoints |
|---|---|---|---|
| [`keycloak`](#keycloak) | Keycloak OIDC/OAuth2 + Admin API | 8080 | `/realms/myrealm/…` |
| [`authelia`](#authelia) | Authelia authentication & forward-auth API | 9091 | `/api/authz/…` |
| [`oauth2`](#oauth2) | Generic OAuth2 / OpenID Connect server | 8080 | `/.well-known/…`, `/oauth2/…` |
| [`github`](#github) | GitHub REST API v3 | 8080 | `/user`, `/repos/…` |
| [`stripe`](#stripe) | Stripe payments API | 8080 | `/v1/customers`, `/v1/payment_intents` |
| [`openai`](#openai) | OpenAI API | 8080 | `/v1/chat/completions`, `/v1/models` |
| [`slack`](#slack) | Slack Web API | 8080 | `/api/chat.postMessage` |
| [`twilio`](#twilio) | Twilio SMS & Voice API | 8080 | `/2010-04-01/Accounts/…` |
| [`sendgrid`](#sendgrid) | SendGrid v3 Email API | 8080 | `/v3/mail/send` |

---

## keycloak

Simulates a Keycloak identity server for the realm **`myrealm`**.

**Configure your app:**
```
KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=myrealm
KEYCLOAK_CLIENT_ID=my-app
```

**Endpoints mocked:**
| Method | Path | Description |
|---|---|---|
| `GET` | `/realms/myrealm/.well-known/openid-configuration` | OIDC discovery |
| `GET` | `/realms/myrealm/protocol/openid-connect/certs` | JWKS (public keys) |
| `POST` | `/realms/myrealm/protocol/openid-connect/token` | Token (password/CC/refresh) |
| `GET` | `/realms/myrealm/protocol/openid-connect/userinfo` | UserInfo |
| `POST` | `/realms/myrealm/protocol/openid-connect/token/introspect` | Token introspection |
| `POST` | `/realms/myrealm/protocol/openid-connect/logout` | Logout |
| `GET` | `/realms/myrealm` | Realm info |
| `GET` | `/admin/realms/myrealm/users` | Admin: list users |
| `GET` | `/admin/realms/myrealm/users/:id` | Admin: get user |
| `POST` | `/admin/realms/myrealm/users` | Admin: create user |
| `DELETE` | `/admin/realms/myrealm/users/:id` | Admin: delete user |
| `GET` | `/admin/realms/myrealm/roles` | Admin: list roles |

**Token response** includes a realistic-looking JWT with subject `user-123456`, username `alice`, email `alice@example.com`.

**State-conditional mocks:** none (stateless by default).

---

## authelia

Simulates Authelia's authentication and forward-auth endpoints.

**Configure Nginx/Traefik:**
```
# Nginx
auth_request /api/authz/forward-auth;

# Traefik
forwardAuth.address=http://localhost:9091/api/authz/forward-auth
```

**Endpoints mocked:**
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/state` | Auth state |
| `GET` | `/api/authz/forward-auth` | Forward auth (200 / 401) |
| `GET` | `/api/verify` | Legacy verify endpoint |
| `POST` | `/api/firstfactor` | Username + password login |
| `POST` | `/api/secondfactor/totp` | TOTP 2FA |
| `GET` | `/api/user/info` | Authenticated user info |
| `POST` | `/api/logout` | Logout |

**Use state to switch auth mode:**
```bash
# All /api/authz/forward-auth requests → 200 (authenticated)
curl -X POST http://localhost:9091/api/state -d '{"authelia_authenticated":"true"}'

# Back to unauthenticated
curl -X DELETE http://localhost:9091/api/state/authelia_authenticated
```

---

## oauth2

Generic OAuth2 / OIDC authorization server following RFC 6749, 7636, 7662, 7009.

**Configure your app:**
```
OAUTH2_ISSUER=http://localhost:8080
OAUTH2_TOKEN_URL=http://localhost:8080/oauth2/token
OAUTH2_JWKS_URI=http://localhost:8080/.well-known/jwks.json
```

**Endpoints mocked:**
| Method | Path | Description |
|---|---|---|
| `GET` | `/.well-known/openid-configuration` | OIDC discovery |
| `GET` | `/.well-known/jwks.json` | JWKS |
| `POST` | `/oauth2/token` | Token (all grant types) |
| `GET` | `/oauth2/userinfo` | UserInfo (RFC 7519) |
| `POST` | `/oauth2/introspect` | Token introspection (RFC 7662) |
| `POST` | `/oauth2/revoke` | Token revocation (RFC 7009) |
| `GET` | `/oauth2/logout` | End session (OIDC RP-initiated) |
| `POST` | `/oauth2/device_authorization` | Device authorization (RFC 8628) |

---

## github

Simulates the GitHub REST API v3 with default user `octocat`, repo `octocat/hello-world`.

**Configure your app:**
```
GITHUB_API_URL=http://localhost:8080
GITHUB_TOKEN=mock-github-pat-token
```

**Endpoints mocked:**
`GET /user`, `GET /users/:login`, `GET /user/repos`, `GET|POST /repos/:owner/:repo/issues`,  
`GET /repos/:owner/:repo/pulls`, `GET /repos/:owner/:repo/commits`,  
`POST /repos/:owner/:repo/dispatches`, `GET /repos/:owner/:repo/actions/runs`,  
`GET /rate_limit` + 401/404/429 error responses.

All responses include standard `X-RateLimit-*` headers.

---

## stripe

Simulates the Stripe v1 API. Uses mock IDs with the prefix `cus_Mock`, `pi_Mock`, etc.

**Configure your app:**
```
STRIPE_API_URL=http://localhost:8080
STRIPE_SECRET_KEY=sk_test_mockly
```

**Endpoints mocked:**
- Customers: `GET|POST /v1/customers`, `GET|POST|DELETE /v1/customers/:id`
- Payment Intents: `POST /v1/payment_intents`, `GET|POST /v1/payment_intents/:id/confirm`
- Payment Methods: `POST /v1/payment_methods`, `POST /v1/payment_methods/:id/attach`
- Subscriptions: `POST /v1/subscriptions`, `DELETE /v1/subscriptions/:id`
- Products & Prices: `GET /v1/products`, `GET /v1/prices`
- Webhooks: `POST /webhook`
- Errors: card_declined, insufficient_funds, authentication_required (402/401)

---

## openai

Simulates the OpenAI API. Responses include realistic token usage fields.

**Configure your app:**
```
OPENAI_API_BASE=http://localhost:8080/v1
OPENAI_API_KEY=sk-mockly-not-a-real-key
```

**Endpoints mocked:**
- `GET /v1/models`, `GET /v1/models/:id`
- `POST /v1/chat/completions` (+ streaming variant)
- `POST /v1/embeddings`
- `POST /v1/images/generations`, `/images/edits`
- `POST /v1/audio/transcriptions`, `/audio/translations`, `/audio/speech`
- `POST /v1/moderations`
- `GET /v1/files`
- Errors: invalid_api_key (401), rate_limit_exceeded (429), context_length_exceeded (400)

---

## slack

Simulates the Slack Web API. Default workspace is `T1234567890` (Mock Team).

**Configure your app:**
```
SLACK_API_URL=http://localhost:8080
SLACK_BOT_TOKEN=xoxb-mock-slack-bot-token-001
```

**Endpoints mocked:**
- `POST /services/…` (incoming webhooks)
- `POST /api/chat.postMessage`, `chat.update`, `chat.delete`
- `GET /api/conversations.list`, `conversations.info`, `conversations.history`
- `GET /api/users.info`, `users.list`
- `POST /api/reactions.add`, `files.upload`
- `POST /api/oauth.v2.access`, `auth.test`
- Errors: not_authed, ratelimited

---

## twilio

Simulates the Twilio REST API. Default Account SID: `ACmockaccountsid001`.

**Configure your app:**
```
TWILIO_ACCOUNT_SID=ACmockaccountsid001
TWILIO_AUTH_TOKEN=mock_auth_token
TWILIO_BASE_URL=http://localhost:8080
```

**Endpoints mocked:**
- `GET /2010-04-01/Accounts/:sid.json` — account info
- `POST /2010-04-01/Accounts/:sid/Messages.json` — send SMS
- `GET /2010-04-01/Accounts/:sid/Messages/:sid.json` — get message
- `GET /2010-04-01/Accounts/:sid/Messages.json` — list messages
- `POST /2010-04-01/Accounts/:sid/Calls.json` — make call
- `POST /v2/Services/:sid/Verifications` — Verify: send OTP
- `POST /v2/Services/:sid/VerificationCheck` — Verify: check OTP
- Errors: invalid_number (400), authentication_required (401)

---

## sendgrid

Simulates the SendGrid v3 API.

**Configure your app:**
```
SENDGRID_API_URL=http://localhost:8080
SENDGRID_API_KEY=SG.mockly-not-a-real-key
```

**Endpoints mocked:**
- `POST /v3/mail/send` — send email (202)
- `GET /v3/templates`, `GET /v3/templates/:id` — dynamic templates
- `GET /v3/verified_senders` — sender verification
- `GET /v3/suppression/unsubscribes`, `bounces`, `spam_reports`
- `GET /v3/stats` — global send statistics
- `POST /v3/validations/email` — email validation
- Errors: unauthorized (401), rate_limited (429)

---

## Customising a Preset

```bash
# Export the preset to a file
mockly preset show keycloak > my-keycloak.yaml

# Edit the file (change realm name, add mocks, etc.)
$EDITOR my-keycloak.yaml

# Start with your custom version
mockly start --config my-keycloak.yaml
```

## Contributing Presets

New presets go in `configs/presets/` as YAML files and must also be:
1. Copied to `internal/presets/` (for `go:embed`)
2. Registered in `internal/presets/presets.go` `All` slice
