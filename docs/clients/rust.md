# Mockly — Rust Client

The Rust client starts, controls, and stops a Mockly process from your Rust tests.

## Install

Add to `Cargo.toml`:

```toml
[dev-dependencies]
mockly-driver = "0.13.0" # x-release-please-version```

## Quickstart

```rust
use mockly_driver::{MocklyServer, ServerOptions, Mock, Request, Response};

#[test]
fn test_my_service() {
    let mut server = MocklyServer::ensure(
        ServerOptions::default(),
        Default::default(),
    ).unwrap();

    server.add_mock(&Mock {
        id: "get-user".into(),
        request: Request { method: "GET".into(), path: "/users/1".into(), ..Default::default() },
        response: Response {
            status: 200,
            body: Some(r#"{"id":1,"name":"Alice"}"#.into()),
            headers: [("Content-Type".into(), "application/json".into())].into(),
            ..Default::default()
        },
    }).unwrap();

    // Point your service under test at server.http_base
    // e.g. "http://127.0.0.1:45678"

    server.stop().unwrap();
}
```

`MocklyServer` implements `Drop` — the process is killed automatically when the server goes out of scope.

## Factory methods

| Method | Description |
|---|---|
| `MocklyServer::ensure(opts, install_opts)` | Downloads the binary if not present, then starts the server. **Recommended for most cases.** |
| `MocklyServer::create(opts)` | Starts using an already-installed binary. Returns `Err` if the binary is not found. |

Both retry up to 3 times on ephemeral port conflicts.

## Configuration

```rust
use mockly_driver::{MocklyServer, ServerOptions, Scenario, ScenarioPatch};

let mut server = MocklyServer::ensure(
    ServerOptions {
        scenarios: vec![
            Scenario {
                id: "payment-fail".into(),
                name: "Payment Failure".into(),
                patches: vec![
                    ScenarioPatch {
                        mock_id: "charge".into(),
                        status: Some(503),
                        body: Some(r#"{"error":"unavailable"}"#.into()),
                        delay: None,
                    },
                ],
            },
        ],
    },
    Default::default(),
).unwrap();
```

## API reference

### Mocks

```rust
use mockly_driver::{Mock, MockRequest, MockResponse, MockResponsePatch};
use std::collections::HashMap;

// Add a mock
server.add_mock(&Mock {
    id: "get-orders".into(),
    request: MockRequest {
        method: "GET".into(),
        path: "/orders".into(),
        headers: HashMap::from([("Authorization".into(), "Bearer *".into())]),
    },
    response: MockResponse {
        status: 200,
        body: Some(r#"[{"id":1}]"#.into()),
        headers: HashMap::from([("Content-Type".into(), "application/json".into())]),
        delay: Some("100ms".into()),
    },
})?;

// Inspect the currently registered mocks
let mocks = server.list_mocks()?;

// Replace a mock definition
let updated = server.update_mock("get-orders", &Mock {
    id: "get-orders".into(),
    request: MockRequest {
        method: "GET".into(),
        path: "/orders".into(),
        headers: HashMap::new(),
    },
    response: MockResponse {
        status: 200,
        body: Some(r#"[{"id":1},{"id":2}]"#.into()),
        headers: HashMap::from([("Content-Type".into(), "application/json".into())]),
        delay: None,
    },
})?;

// Patch only the response fields you want to change
let patched = server.patch_mock("get-orders", &MockResponsePatch {
    status: Some(201),
    body: Some("[]".into()),
    headers: Some(HashMap::from([("X-Mock-Version".into(), "v2".into())])),
    delay: Some("250ms".into()),
})?;

// Remove a mock
server.delete_mock("get-orders")?;
```

### Scenarios

```rust
use mockly_driver::{Scenario, ScenarioPatch};

let created_scenario = server.create_scenario(&Scenario {
    id: "slow-checkout".into(),
    name: "Slow checkout".into(),
    description: Some("Used for retry-path tests".into()),
    patches: vec![ScenarioPatch {
        mock_id: "charge".into(),
        status: Some(503),
        body: None,
        headers: None,
        delay: Some("750ms".into()),
        disabled: None,
    }],
})?;

let scenarios = server.list_scenarios()?;
let loaded_scenario = server.get_scenario("slow-checkout")?;

let updated_scenario = server.update_scenario("slow-checkout", &Scenario {
    name: "Slow checkout v2".into(),
    ..loaded_scenario.clone()
})?;

// Activate a scenario before exercising your service
server.activate_scenario("slow-checkout")?;
let active_scenarios = server.list_active_scenarios()?;
println!("{:?}", active_scenarios.active);

// Deactivate or delete it when you're done
server.deactivate_scenario("slow-checkout")?;
server.delete_scenario("slow-checkout")?;
```

### Call verification

```rust
let summary = server.wait_for_calls("get-orders", 2, 5)?;
assert_eq!(summary.count, 2);

let latest_calls = server.get_calls("get-orders")?;
println!("{}", latest_calls.calls[0].path);

server.clear_calls("get-orders")?;
server.clear_all_calls()?;
```

### State

```rust
let state = server.get_state()?;
println!("{:?}", state.get("order-status"));

let updated_state = server.set_state(&HashMap::from([
    ("order-status".into(), "pending".into()),
    ("retry-count".into(), "1".into()),
]))?;
println!("{:?}", updated_state.get("retry-count"));

server.delete_state("retry-count")?;
```

### Logs

```rust
let all_logs = server.get_logs(None)?;
let matched_logs = server.get_logs(Some("get-orders"))?;

let total_logs = server.get_logs_count(None)?;
let matched_count = server.get_logs_count(Some("get-orders"))?;
println!("{} {}", total_logs, matched_count);
println!("{:?}", all_logs.first().map(|entry| &entry.path));
println!("{:?}", matched_logs.first().and_then(|entry| entry.matched_id.as_deref()));

server.clear_logs()?;
```

### Fault injection

```rust
use mockly_driver::FaultConfig;

// Add latency and override status codes on all requests
server.set_fault(&FaultConfig {
    enabled: true,
    delay: Some("500ms".into()),
    status: Some(503),
    error_rate: Some(0.5), // 50% of requests
}).unwrap();

// Remove the fault
server.clear_fault().unwrap();
```

### Reset and stop

```rust
// Reset all dynamic mocks, active scenarios, and faults; keeps startup config
server.reset().unwrap();

// Kill the process (also called automatically on drop)
server.stop().unwrap();
```

## Integration with `#[test]`

### Shared server via `OnceLock`

```rust
use mockly_driver::{MocklyServer, ServerOptions};
use std::sync::{Mutex, OnceLock};

static SERVER: OnceLock<Mutex<MocklyServer>> = OnceLock::new();

fn server() -> &'static Mutex<MocklyServer> {
    SERVER.get_or_init(|| {
        let s = MocklyServer::ensure(ServerOptions::default(), Default::default()).unwrap();
        Mutex::new(s)
    })
}

#[test]
fn returns_user() {
    let mut s = server().lock().unwrap();
    s.reset().unwrap();
    s.add_mock(/* ... */).unwrap();
    // ...
}
```

### Per-test server (fully isolated)

```rust
#[test]
fn isolated_test() {
    let mut server = MocklyServer::ensure(
        ServerOptions::default(),
        Default::default(),
    ).unwrap();

    server.add_mock(/* ... */).unwrap();
    // server is stopped when it drops at end of scope
}
```

## Server properties

| Field | Description |
|---|---|
| `server.http_base` | Base URL of the mock HTTP server, e.g. `http://127.0.0.1:45123` |
| `server.api_base` | Base URL of the management API, e.g. `http://127.0.0.1:45124` |
| `server.http_port` | Numeric HTTP port |
| `server.api_port` | Numeric API port |

## Testcontainers

Mockly also ships a Docker-backed Rust testcontainers crate: `mockly-testcontainers`.

Use it instead of the driver when you want Docker-managed lifecycle, no local binary download, and the same container image in local tests and CI.

### Install

```toml
[dev-dependencies]
mockly-testcontainers = "0.13.0" # x-release-please-version
reqwest = { version = "0.12", features = ["blocking"] }
testcontainers = { version = "0.23", features = ["blocking", "http_wait"] }
```

### Example

```rust
use mockly_testcontainers::{Mock, MockRequest, MockResponse, MocklyContainer, MocklyImage};
use testcontainers::runners::SyncRunner;

#[test]
fn returns_user_from_container() -> Result<(), Box<dyn std::error::Error>> {
    let container = MocklyContainer::new(MocklyImage::default().start()?);

    container.add_mock(&Mock {
        id: "get-user".into(),
        request: MockRequest { method: "GET".into(), path: "/users/1".into(), headers: Default::default() },
        response: MockResponse { status: 200, body: Some(r#"{"id":1}"#.into()), headers: Default::default(), delay: None },
    })?;

    let response = reqwest::blocking::get(format!("{}/users/1", container.http_base()))?;
    assert_eq!(response.status().as_u16(), 200);
    assert_eq!(response.text()?, r#"{"id":1}"#);
    Ok(())
}
```

### Key API

- `MocklyImage::with_inline_config(yaml)`
- `MocklyContainer::http_base()` / `api_base()`
- `add_mock`, `delete_mock`, `reset`
- `activate_scenario`, `deactivate_scenario`
- `set_fault`, `clear_fault`

### Requirements

- Rust 1.85+
- Docker

See `clients/rust-testcontainers/README.md` for the full module reference.
