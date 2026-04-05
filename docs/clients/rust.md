# Mockly — Rust Client

The Rust client starts, controls, and stops a Mockly process from your Rust tests.

## Install

Add to `Cargo.toml`:

```toml
[dev-dependencies]
mockly-driver = "0.4"
```

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
use mockly_driver::{Mock, Request, Response};
use std::collections::HashMap;

// Add a mock
server.add_mock(&Mock {
    id: "get-orders".into(),
    request: Request {
        method: "GET".into(),
        path: "/orders".into(),
        headers: [("Authorization".into(), "Bearer *".into())].into(),
    },
    response: Response {
        status: 200,
        body: Some(r#"[{"id":1}]"#.into()),
        headers: [("Content-Type".into(), "application/json".into())].into(),
        delay: Some("100ms".into()),
    },
}).unwrap();

// Remove a mock
server.delete_mock("get-orders").unwrap();
```

### Scenarios

```rust
// Activate a pre-configured scenario
server.activate_scenario("payment-fail").unwrap();

// Deactivate it
server.deactivate_scenario("payment-fail").unwrap();
```

### Fault injection

```rust
use mockly_driver::FaultConfig;

// Add latency and override status codes on all requests
server.set_fault(&FaultConfig {
    enabled: true,
    delay: Some("500ms".into()),
    status_override: Some(503),
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
