# mockly-testcontainers

Run Mockly in Docker-backed Rust tests with `testcontainers`.

The crate starts `ghcr.io/dever-labs/mockly:latest`, waits for the management API to be ready, and exposes helper methods for mocks, scenarios, faults, and logs.

## Requirements

- Rust 1.85+
- Docker

## Install

Add this to `Cargo.toml`:

```toml
[dev-dependencies]
mockly-testcontainers = "0.12.4" # x-release-please-version
reqwest = { version = "0.12", features = ["blocking"] }
testcontainers = { version = "0.23", features = ["blocking", "http_wait"] }
```

## Quickstart

```rust
use mockly_testcontainers::{Mock, MockRequest, MockResponse, MocklyContainer, MocklyImage};
use testcontainers::runners::SyncRunner;

#[test]
fn returns_mocked_user() -> Result<(), Box<dyn std::error::Error>> {
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

## When to use the testcontainers module

Use `mockly-testcontainers` when you want Docker-managed lifecycle, no native binary download, or the same Mockly image in local tests and CI.

Use `mockly-driver` when you prefer starting the native Mockly binary directly from the test process.

## Core types

### `MocklyImage`

Use `MocklyImage` to configure how the Docker image is started.

| Method | Description |
|---|---|
| `MocklyImage::default()` | Use `ghcr.io/dever-labs/mockly:latest` with the default config. |
| `with_tag(tag)` | Override the image tag. |
| `with_inline_config(yaml)` | Replace `/config/mockly.yaml` with inline YAML. |

### `MocklyContainer`

`MocklyContainer` wraps `testcontainers::core::Container<MocklyImage>` and provides Mockly-specific helpers.

| Method | Description |
|---|---|
| `new(container)` | Wrap a started Testcontainers container. |
| `inner()` / `into_inner()` | Access the underlying Testcontainers container. |
| `http_port()` / `api_port()` | Read the mapped host ports. |
| `http_base()` / `api_base()` | Read the mock HTTP and management API base URLs. |
| `add_mock(&Mock)` | Register a dynamic HTTP mock. |
| `delete_mock(id)` | Delete a mock by ID. |
| `reset()` | Remove dynamic mocks, deactivate scenarios, and clear faults. |
| `activate_scenario(id)` | Activate a configured scenario. |
| `deactivate_scenario(id)` | Deactivate a configured scenario. |
| `set_fault(&FaultConfig)` | Apply a global HTTP fault. |
| `clear_fault()` | Remove the active fault. |
| `get_logs()` | Read request logs as JSON. |
| `clear_logs()` | Clear stored request logs. |

### Custom YAML config

```rust
use mockly_testcontainers::{MocklyContainer, MocklyImage};
use testcontainers::runners::SyncRunner;

let container = MocklyContainer::new(
    MocklyImage::default()
        .with_inline_config(r#"mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
"#)
        .start()?,
);
```
